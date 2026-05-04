package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"log"
	"net/http"
	"io"	
	"os"
	"time"
	"strconv"
	"strings"

	"github.com/google/uuid"
	_"github.com/lib/pq"
	"github.com/gregcozza-ai/blog_agg/internal/config"
	"github.com/gregcozza-ai/blog_agg/internal/commands"
	"github.com/gregcozza-ai/blog_agg/internal/state"
	"github.com/gregcozza-ai/blog_agg/internal/database"
)

// RSSFeed represents the structure of an RSS feed
type RSSFeed struct {
	Channel struct {
		Title		string		`xml:"title"`
		Link		string		`xml:"link"`
		Description	string		`xml:"description"`
		Item		[]RSSItem	`xml:"item"`
	} `xml:"channel"`
}

// RSSItem represents a single item in an RSS feed
type RSSItem struct {
	Title		string `xml:"title"`
	Link		string `xml:"link"`
	Description	string `xml:"description"`
	PubDate		string `xml:"pubDate"`
}
// middlewareLoggedIn is a high-order function that wraps handlers requiring a logged-in user.
func middlewareLoggedIn(handler func(s *state.State, cmd state.Command, user database.User) error) func(*state.State, state.Command) error {
	return func(s *state.State, cmd state.Command) error {
		if s.Cfg.CurrentUser == "" {
			return fmt.Errorf("no current user set")
		}
		user, err := s.Db.GetUser(context.Background(), s.Cfg.CurrentUser)
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		return handler(s, cmd, user)
	}
}

// fetchFeed fetches and parses an RSS feed from the given URL
func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "gator")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var feed RSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	// Unescape HTML entities in titles and descriptions
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}
	
	return &feed, nil

}


func handlerLogin(s *state.State, cmd state.Command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("username required")
	}
	// Check if user exists in database
	user, err := s.Db.GetUser(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("user %q does not exist", cmd.Args[0])
	}
	
	// Update config with new user
	if err := s.Cfg.SetUser(cmd.Args[0]); err != nil {
		return err
	}
	
	fmt.Printf("User set to %q (ID: %s)\n", user.Name, user.ID)
	fmt.Printf("(Config file update: ~/.gatorconfig.json now has current_user_name: %q)\n", user.Name)
	return nil
}

func handlerRegister(s *state.State, cmd state.Command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("username required")
	}

	username := cmd.Args[0]

	// Check if user already exists
	_, err := s.Db.GetUser(context.Background(), username)
	if err == nil {
		return fmt.Errorf("user %q already exists", username)
	}

	// Create new user
	user, err := s.Db.CreateUser(context.Background(), database.CreateUserParams {
		ID:		uuid.New(),
		CreatedAt:	time.Now(),
		UpdatedAt:	time.Now(),
		Name:		username,
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Update config with new user
	if err := s.Cfg.SetUser(username); err != nil {
		return err
	}

	fmt.Printf("User created: %s (ID: %s)\n", user.Name, user.ID)
	return nil
}

func handlerReset(s *state.State, cmd state.Command) error {
	// Delete in correct order (child tables first)
	if err := s.Db.DeleteFeedFollows(context.Background()); err != nil {
		return fmt.Errorf("failed to delete feed follows: %w", err)
	}
	if err := s.Db.DeleteFeeds(context.Background()); err != nil {	
		return fmt.Errorf("failed to delete feeds: %w", err)
	}
	if err := s.Db.DeleteAllUsers(context.Background()); err != nil {
		return fmt.Errorf("failed to delete all users: %w", err)	
	}
	fmt.Println("Database reset successfully")
	return nil
}

func handlerUsers(s *state.State, cmd state.Command) error {
	users, err := s.Db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	for _, user := range users {
		if user.Name == s.Cfg.CurrentUser {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}
	return nil
}

func scrapeFeeds(s *state.State) {
	// Get the next feed to fetch
	feed, err := s.Db.GetNextFeedToFetch(context.Background())
	if err != nil {
		fmt.Printf("Failed to get next feed to fetch: %v\n", err)
		return
	}

	// Mark the feed as fetched
	err = s.Db.MarkFeedFetched(context.Background(), feed.ID)
	if err != nil {
		fmt.Printf("Failed to mark feed as fetched: %v\n", err)
		return
	}

	// Fetch the feed
	fmt.Printf("Fetching feed: %s (URL: %s)\n", feed.Name, feed.Url)
	feedData, err := fetchFeed(context.Background(), feed.Url)
	if err != nil { 
		fmt.Printf("Failed to fetch feed %s: %v\n", feed.Name, err)
		return
	}

	// Save each post to the database
	for _, item := range feedData.Channel.Item {
		// Parse published date with multiple formats
		publishedAt := time.Time{}
		if item.PubDate != "" {
			layouts := []string{
				time.RFC1123,
				time.RFC3339,
				time.RFC822,
				"Mon, 02 Jan 2006 15:04:05 -0700",
			}
			for _, layout := range layouts {
				if t,err := time.Parse(layout, item.PubDate); err == nil {
					publishedAt = t
					break
				}
			}
		}

		// Create the post (ignore duplicates)
		err = s.Db.CreatePost(context.Background(), database.CreatePostParams{
			ID:		uuid.New(),
			CreatedAt:	time.Now(),
			UpdatedAt:	time.Now(),
			Title:		item.Title,
			Url:		item.Link,
			Description:	sql.NullString{String: item.Description, Valid: item.Description != ""},
			PublishedAt:	sql.NullTime{Time: publishedAt, Valid: !publishedAt.IsZero()}, 
			FeedID:		feed.ID,
		})
		if err != nil {
			// Handle duplicate URL (unique constraint)
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint"){
				fmt.Printf("Post already exists: %s\n", item.Title) // Optional: disable to avoid clutter
			} else {
				fmt.Printf("Failed to save post: '%s': %v\n", item.Title, err)
			}
		}
	}
}

func handlerBrowse(s *state.State, cmd state.Command, user database.User) error {
	limit := 2 // Default limit
	if len(cmd.Args) > 0 {
		n, err := strconv.Atoi(cmd.Args[0])
		if err != nil {
			return fmt.Errorf("invalid limit: %w", err)
		}
		if n <= 0 {
			return fmt.Errorf("limit must be positive")
		}
		limit = n
	}

	// Get posts for the current user
	posts, err := s.Db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID:		user.ID, 
		Limit:		int32(limit),
	})
	if err != nil {
		return fmt.Errorf("failed to get posts: %w", err)
	}

	if len(posts) == 0 {
		fmt.Println("No posts found")
		return nil
	}

	fmt.Println("\nPosts:")
	for _, post := range posts {
		fmt.Printf("* %s (from %s)\n", post.Title, post.FeedName)
		fmt.Printf("  URL: %s\n", post.Url)
		if post.Description.Valid {
			fmt.Printf("  Description: %s\n", post.Description.String)
		}
		if post.PublishedAt.Valid {
			fmt.Printf("  Published: %s\n", post.PublishedAt.Time.Format("2006-01-02 15:04"))
		}
		fmt.Println()
	}
	return nil
}

func handlerAgg(s *state.State, cmd state.Command, user database.User) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("usage: agg <duration>")
	}
	
	duration, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return fmt.Errorf("invalid duration %w", err)
	}

	fmt.Printf("Collecting feeds every %s\n", duration)

	ticker := time.NewTicker(duration)
	// Run immediately
	scrapeFeeds(s)
	for range ticker.C {	
		scrapeFeeds(s)
	}
	return nil
}

func handlerAddFeed(s *state.State, cmd state.Command, user database.User) error {
	if len(cmd.Args) < 2 {
		return fmt.Errorf("usage: addfeed <name> <url>")
	}

	name := cmd.Args[0]
	url := cmd.Args[1]

	// Check if the feed already exists
	_, err := s.Db.GetFeedByURL(context.Background(), url)
	if err == nil {
		return fmt.Errorf("feed with URL %q already exists", url)
	}

	// Create the feed
	feed, err := s.Db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:		uuid.New(),
		CreatedAt:	time.Now(),
		UpdatedAt:	time.Now(),
		Name:		name,
		Url:		url,
	})
	if err != nil {
		return fmt.Errorf("failed to create feed: %w", err)
	}

	// Automatically create feed follow for current user
	_, err = s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams {
		ID:		uuid.New(),
		CreatedAt:	time.Now(),
		UpdatedAt:	time.Now(),
		FeedID:		feed.ID,
		UserID:		user.ID,	
	})
	if err != nil {
		return fmt.Errorf("failed to create feed follow: %w", err)
	}
	 
	fmt.Printf("Feed created: %s (URL: %s)\n", feed.Name, feed.Url)
	return nil
}	

// New handler for listing all feeds
func handlerFeeds(s *state.State, cmd state.Command) error {
	feeds, err := s.Db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get feeds: %w", err)
	}
	
	if len(feeds) == 0 {
		fmt.Println("No feeds found")
		return nil
	}

	fmt.Println("\nAll Feeds:")
	for _, feed := range feeds {
		fmt.Printf("* %s (%s)\n", feed.Name, feed.Url)
	}
	return nil
}

func handlerFollow(s *state.State, cmd state.Command, user database.User) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("usage: follow <feed_url>")
	}

	feedURL := cmd.Args[0]

	// Check if feed exists
	feed, err := s.Db.GetFeedByURL(context.Background(), feedURL)
	fmt.Printf("feed retuned: %s\n", feed)
	if err != nil {
		return fmt.Errorf("feed with URL %q not found", feedURL)
	}

	// Create feed follow
	_, err = s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams {
		ID:		uuid.New(),
		CreatedAt:	time.Now(),
		UpdatedAt:	time.Now(),
		FeedID:		feed.ID,
		UserID:		user.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to create feed follow: %w", err)
	}

	fmt.Printf("Now following feed: %s (URL: %s)\n", feed.Name, feed.Url)
	return nil
}

func handlerFollowing(s *state.State, cmd state.Command, user database.User) error {
	// Get all feed follows for this user
	feedFollows, err := s.Db.GetFeedFollowsByUser(context.Background(), user.ID)
	if err != nil {	
		return fmt.Errorf("failed to get feed follows: %w", err)
	}

	if len(feedFollows) == 0 {
		fmt.Println("No feeds followed")
		return nil
	}
	
	fmt.Println("\nFollowing:")
	for _,ff := range feedFollows {
		fmt.Printf("* %s (%s)\n", ff.Name, ff.Url)
	}
	return nil
}

func handlerUnfollow(s *state.State, cmd state.Command, user database.User) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("usage: unfollow <feed_url>")
	}

	feedURL := cmd.Args[0]

	// Check if feed exists
	feed, err := s.Db.GetFeedByURL(context.Background(), feedURL)
	if err != nil {
		return fmt.Errorf("feed with URL %q not found", feedURL)
	}

	// Delete the feed follow record
	err = s.Db.DeleteFeedFollowsUser(context.Background(), database.DeleteFeedFollowsUserParams{
	FeedID: feed.ID, 
	UserID: user.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to unfollow feed: %w", err)
	}

	fmt.Printf("Unfollowd feed: %s (URL: %s)\n", feed.Name, feed.Url)
	return nil
}
 
func main() {
	// Read config file
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	// Open database connection
	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create database queries
	dbQueries := database.New(db)

	// Create state with config
	s := &state.State{
		Db: dbQueries,
		Cfg: &cfg,
	}

	// Initalize commands
	cmd := commands.New()
	cmd.Register("login", handlerLogin)
	cmd.Register("register", handlerRegister)
	cmd.Register("reset", handlerReset)
	cmd.Register("users", handlerUsers)
	cmd.Register("agg", middlewareLoggedIn(handlerAgg))
	cmd.Register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmd.Register("feeds", handlerFeeds)
	cmd.Register("follow", middlewareLoggedIn(handlerFollow))
	cmd.Register("following", middlewareLoggedIn(handlerFollowing))
	cmd.Register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmd.Register("browse", middlewareLoggedIn(handlerBrowse))

	// Check arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: gator <command> <username>")
		fmt.Println("Available commands: login, register, reset, users, agg, addfeed, feeds, follow, following, unfollow")
		os.Exit(1)
	}

	//Parse command
	cmdName := os.Args[1]
	args := os.Args[2:]

	// Check for required argument for login / regiseter
	if (cmdName == "login" || cmdName == "regiser") && len(args) == 0 {
		fmt.Printf("Error: %s command requires a username\n", cmdName)
		os.Exit(1)
	}

	if cmdName == "addfeed" && len(args) < 2 {
		fmt.Println("Error: addfeed requires two arguments: <name> <url>")
		os.Exit(1)
	}

	if cmdName == "follow" && len(args) < 1 {
		fmt.Println("Error: follow requires a feed URL")
		os.Exit(1)
	}

	// Run command
	err = cmd.Run(s, state.Command{
		Name: cmdName,
		Args: args,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
