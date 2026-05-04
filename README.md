# Gator RSS Aggregator

Gator is a command-line RSS feed aggregator that lets you follow your favorite blogs and news sources, then
view their latest posts in one place. It's built with Go, PostgreSQL, and SQLC.

## Features

- Follow RSS feeds from multiple sources
- Automatically fetch and save posts to your database
- Browse your followed posts with a simple command
- Run in the background while you work
- Support for multiple users

## Prerequisites

Before installing Gator, you'll need to have:

1. **PostgreSQL** installed and running (version 12+)
2. **Go** installed (version 1.18+)

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/gregcozza-ai/blog_agg.git
   cd blog_agg
   ```

2. Install Gator using `go install`:
   ```bash
   go install .
   ```

3. Add Gator to your PATH (if not already done):
   ```bash
   # For Bash
   echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc
   source ~/.bashrc

   # For Zsh
   echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
   source ~/.zshrc
   ```

## Setup

### 1. Configure PostgreSQL

Gator needs a PostgreSQL database to run. First, create a database:

```bash
# Start PostgreSQL (if not running)
sudo systemctl start postgresql

# Create a database
sudo -u postgres createdb gator
```

### 2. Set up the configuration file

Gator uses a config file at `~/.gatorconfig.json`. Set up your database connection and first user:

```bash
# Create config file with your database URL
echo '{"db_url": "postgres://localhost/gator?sslmode=disable", "current_user_name": "kahya"}' >
~/.gatorconfig.json
```

> **Note**: Replace `kahya` with your preferred username.

### 3. Run the program

```bash
gator
```

## Usage

### Basic Commands

| Command | Description | Example |
|---------|-------------|---------|
| `register <username>` | Create a new user | `gator register kahya` |
| `login <username>` | Switch to a different user | `gator login kahya` |
| `users` | List all users | `gator users` |
| `reset` | Reset the database (delete all data) | `gator reset` |

### Feed Management

| Command | Description | Example |
|---------|-------------|---------|
| `addfeed <name> <url>` | Add a new RSS feed | `gator addfeed "TechCrunch" "https://techcrunch.com/feed/"` |
| `follow <url>` | Follow a feed you've added | `gator follow "https://techcrunch.com/feed/"` |
| `following` | View all feeds you're following | `gator following` |

### Aggregation

| Command | Description | Example |
|---------|-------------|---------|
| `agg <duration>` | Start continuous feed fetching | `gator agg 1m` |
| `browse [limit]` | View your latest posts | `gator browse 5` |

## Example Workflow

```bash
# Reset database (start fresh)
gator reset

# Create user
gator register kahya

# Add feeds
gator addfeed "TechCrunch" "https://techcrunch.com/feed/"
gator addfeed "Hacker News" "https://news.ycombinator.com/rss"

# Follow feeds
gator follow "https://techcrunch.com/feed/"
gator follow "https://news.ycombinator.com/rss"

# Start fetching feeds (every minute)
gator agg 1m

# View your posts (default 2 posts)
gator browse

# View 5 posts
gator browse 5
```

## Stopping the Program

Press `Ctrl+C` to stop the continuous feed fetching.

## Troubleshooting

- **Database connection issues**: Make sure PostgreSQL is running and your database URL is correct
- **"User not found"**: You need to register a user first with `register`
- **"Feed not found"**: Make sure you've added the feed with `addfeed` before following it

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on GitHub.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
