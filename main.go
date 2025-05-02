package main

import _ "github.com/lib/pq"

import (
	"fmt"
	"os"
	"io"
	"github.com/smythg4/go-gator/internal/config"
	"github.com/smythg4/go-gator/internal/database"
	"database/sql"
	"time"
	"context"
	"github.com/google/uuid"
	"net/http"
	"encoding/xml"
	"html"
	"github.com/lib/pq"
	"strconv"
	"strings"
	"regexp"
)

type State struct {
	db		*database.Queries
	cfg 	*config.Config
}

type Command struct {
	Name	string
	Args 	[]string
}

type Commands struct {
	handlers		map[string]func(*State, Command) error 
}

func initCommands() Commands {
	var cmds Commands
	cmds.handlers = make(map[string]func(*State, Command) error, 5)
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))
	return cmds
}

func (c *Commands) run(s *State, cmd Command) error {
	handler, ok := c.handlers[cmd.Name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
	return handler(s, cmd)
}

func (c *Commands) register(name string, f func(*State, Command) error) {
	c.handlers[name] = f
}

func middlewareLoggedIn(handler func(s *State, cmd Command, user database.User) error) func (*State, Command) error {
	// accepts a handler function that takes a user and returns a regular handler function signature
	return func(s *State, cmd Command) error {
		currentUser, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			fmt.Printf("Error: user %s does not exist: %w", s.cfg.CurrentUserName, err)
			os.Exit(1)
			return nil
		}
		return handler(s, cmd, currentUser)
	}
}

func handlerLogin(s *State, cmd Command) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("Username required")
	}

	username := cmd.Args[0]
	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		fmt.Printf("Error: user %s does not exist: %w", username, err)
		os.Exit(1)
		return nil
	}

	err = s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("Error setting user %s: %w", username, err)
	}
	fmt.Printf("User has been set to %s\n", username)
	return nil
}

func handlerRegister(s *State, cmd Command) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("Username required")
	}

	username := cmd.Args[0]
	params := database.CreateUserParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:	username,
	}
	new_user, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		fmt.Printf("Error creating user: %v\n", err)
		os.Exit(1)
		return nil
	}

	err = s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("Error setting user %s: %w", username, err)
	}
	fmt.Println("User created:", new_user)
	return nil
}

func handlerReset(s *State, cmd Command) error {
	err := s.db.ResetDB(context.Background())
	if err != nil {
		fmt.Printf("Error resetting the database: %v", err)
		os.Exit(1)
		return nil
	}
	return nil
}

func handlerUsers(s *State, cmd Command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Error retrieving users: %w", err)
	}

	if len(users) < 1 {
		fmt.Println("No users in database.")
	}

	currentuser := s.cfg.CurrentUserName

	for i := 0; i < len(users); i++ {
		fmt.Printf("%s",users[i].Name)
		if users[i].Name == currentuser {
			fmt.Printf(" (current)\n")
		} else {
			fmt.Printf("\n")
		}
	}

	return nil
}

func handlerAgg(s *State, cmd Command) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("Must provide time between requests argument")
	}
	time_between_reqs := cmd.Args[0]
	duration, err := time.ParseDuration(time_between_reqs)
	if err != nil {
		return fmt.Errorf("Error parsing time duration %s: %w", time_between_reqs, err)
	}
	fmt.Println("Collecting feeds every", duration)

	ticker := time.NewTicker(duration)
	for ;; <-ticker.C {
		scrapeFeeds(context.Background(), s)
	}
	return nil
}

func handlerAddFeed(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) < 2 {
		return fmt.Errorf("Feed name and URL are required")
	}

	feedname := cmd.Args[0]
	feedURL := cmd.Args[1]

	params := database.CreateFeedParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:	feedname,
		Url:	feedURL,
		UserID:	user.ID,
	}

	new_feed, err := s.db.CreateFeed(context.Background(), params)
	if err != nil {
		fmt.Printf("Error creating feed: %v\n", err)
		os.Exit(1)
		return nil
	}
	fmt.Println("Feed created:", new_feed)
	// add feed follow
	feed_params := database.CreateFeedFollowParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:		user.ID,
		FeedsID:	new_feed.ID,
	}

	row, err := s.db.CreateFeedFollow(context.Background(), feed_params)
	if err != nil {
		return fmt.Errorf("Error creating follow: %w", err)
	}

	fmt.Printf("Success adding feed: %s - %s", row.UserName, row.FeedName)
	return nil
}

func handlerFeeds(s *State, cmd Command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("Error retrieving feeds: %w", err)
	}

	if len(feeds) < 1 {
		fmt.Println("No feeds in database.")
	}

	for i := 0; i < len(feeds); i++ {
		fmt.Printf("%s - ",feeds[i].Name)
		fmt.Printf("%s - ", feeds[i].Url)
		user, err := s.db.GetUserByID(context.Background(), feeds[i].UserID)
		if err != nil {
			return fmt.Errorf("Error finding username: %w", err)
		}
		fmt.Printf("%s\n", user.Name)
	}

	return nil
}

func handlerFollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("URL required to follow")
	}

	feed, err := s.db.GetFeedByURL(context.Background(),cmd.Args[0])
	if err != nil {
		return fmt.Errorf("Error finding feed id: %w", err)
	}
	params := database.CreateFeedFollowParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:		user.ID,
		FeedsID:	feed.ID,
	}

	row, err := s.db.CreateFeedFollow(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Error creating follow: %w", err)
	}

	fmt.Printf("Success adding feed: %s - %s", row.UserName, row.FeedName)

	return nil
}

func handlerFollowing(s *State, cmd Command, user database.User) error {

	rows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("Error fetch user feeds for %s: %w", s.cfg.CurrentUserName, err)
	}
	if len(rows) < 1 {
		fmt.Println("No feeds found for current user.")
		return nil
	}
	for i := 0; i < len(rows); i++ {
		fmt.Println(rows[i].FeedName)
	}
	return nil
}

func handlerUnfollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) < 1 {
		return fmt.Errorf("Feed URL required to unfollow.")
	}
	feed, err := s.db.GetFeedByURL(context.Background(),cmd.Args[0])
	if err != nil {
		return fmt.Errorf("Error retrieving feed id for %s: %w", cmd.Args[0], err)
	}

	params := database.DeleteFeedFollowParams{
		UserID:  user.ID,
		FeedsID: feed.ID,
	}

	err = s.db.DeleteFeedFollow(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Error deleting feed follow: %w", err)
	}

	return nil
}

func handlerBrowse(s *State, cmd Command, user database.User) error {
	var fetch_limit int32
	if len(cmd.Args) < 1 {
		fetch_limit = 2
	} else {
		limit, err := strconv.Atoi(cmd.Args[0])
		if err != nil {
			return fmt.Errorf("Invalid limit parameter: %w", err)
		}
		fetch_limit = int32(limit)
	}
	

	params := database.GetPostsForUserParams{
		UserID:    user.ID,
		Limit: fetch_limit,
	}

	posts, err := s.db.GetPostsForUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Error fetching posts: %w", err)
	}

	if len(posts) < 1 {
		fmt.Println("No posts to print.")
		return nil
	}


	for i, post := range posts {
		feed, err := s.db.GetFeedByID(context.Background(), post.FeedID)
		if err != nil {
			return fmt.Errorf("error getting feed data: %w", err)
		}

		publishedAt := "Unknown"
		if post.PublishedAt.Valid {
			publishedAt = post.PublishedAt.Time.Format("Jan 02, 2006")
		}

		if i > 0 {
			fmt.Println("\n" + strings.Repeat("-",80) + "\n")
		}
		fmt.Printf("Title: \033[1m%s\033[0m\n", post.Title)
        fmt.Printf("Feed: \033[36m%s\033[0m\n", feed.Name)
        fmt.Printf("Published: \033[33m%s\033[0m\n", publishedAt)
        fmt.Printf("URL: \033[4;34m%s\033[0m\n", post.Url)
        if post.Description.Valid && post.Description.String != "" {
            fmt.Println("\nDescription:")
            
            // Word wrap the description to make it more readable
            desc := post.Description.String
            // Strip HTML tags if present (simple implementation)
            desc = regexp.MustCompile("<[^>]*>").ReplaceAllString(desc, "")
            
            // Wrap text at 80 characters
            wrapped := ""
            words := strings.Fields(desc)
            lineLength := 0
            
            for _, word := range words {
                if lineLength+len(word)+1 > 80 {
                    wrapped += "\n" + word + " "
                    lineLength = len(word) + 1
                } else {
                    wrapped += word + " "
                    lineLength += len(word) + 1
                }
            }
            
            fmt.Println(wrapped)
        }
    }
    
    fmt.Println("\n" + strings.Repeat("=", 80))
    fmt.Printf("\nShowing %d of your latest posts. Run 'browse <limit>' to see more.\n\n", len(posts))
    
    return nil
}
 
type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func unescapeRssFeed(rss *RSSFeed) error { 
	rss.Channel.Title = html.UnescapeString(rss.Channel.Title)
	rss.Channel.Description = html.UnescapeString(rss.Channel.Description)
	for i := 0; i < len(rss.Channel.Item); i++ {
		rss.Channel.Item[i].Title = html.UnescapeString(rss.Channel.Item[i].Title)
		rss.Channel.Item[i].Description = html.UnescapeString(rss.Channel.Item[i].Description)
	}
	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		"GET", 
		feedURL, 
		nil)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("Error generating GET request: %w", err)
	}

	req.Header.Set("User-Agent", "gator")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("Error making GET request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("Error reading GET response: %w", err)
	}

	if resp.StatusCode > 299 {
		return &RSSFeed{}, fmt.Errorf("Unsuccessful response status code: %v", resp.Status)
	}

	var rss RSSFeed 
	err = xml.Unmarshal(data, &rss)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("Error unmarshaling data: %w", err)
	}

	err = unescapeRssFeed(&rss)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("Error removing escaped chars: %w", err)
	}

	//fmt.Println(rss)

	return &rss, nil
}

func parsePublishedAt(dateStr string) (time.Time, error) {
    // Common RSS date formats
    formats := []string{
        time.RFC1123Z,  // "Mon, 02 Jan 2006 15:04:05 -0700"
        time.RFC1123,   // "Mon, 02 Jan 2006 15:04:05 MST" 
        "2006-01-02T15:04:05Z", // ISO8601
        "2006-01-02T15:04:05-07:00",
    }
    
    for _, format := range formats {
        if t, err := time.Parse(format, dateStr); err == nil {
            return t, nil
        }
    }
    
    // If we can't parse the date, return the zero time
    return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}

func scrapeFeeds(ctx context.Context, s *State) error {
	feed, err := s.db.GetNextFeedToFetch(ctx)
	fmt.Printf("Fetching feed: ID=%s, URL=%s\n", feed.ID, feed.Url)
	if err != nil {
		return fmt.Errorf("Error finding next feed to fetch: %w", err)
	}
	rss_data, err := fetchFeed(ctx, feed.Url)
	
	err = s.db.MarkFeedFetched(ctx, feed.ID)
	if err != nil {
		return fmt.Errorf("Error marking feed as fetched: %w", err)
	}

	if err != nil {
		return fmt.Errorf("Error fetching feed: %w", err)
	}
	fmt.Printf("Found %d items in feed\n", len(rss_data.Channel.Item))

	if len(rss_data.Channel.Item) < 1 {
		fmt.Println("No items in the RSS feed.")
		return nil
	}

	for i := 0; i < len(rss_data.Channel.Item); i++ {
		//fmt.Println(rss_data.Channel.Item[i].Title)
		item := rss_data.Channel.Item[i]
		
		publishedAt, err := parsePublishedAt(item.PubDate)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
			publishedAt = time.Time{} // will be considered null time in SQL
		}

		params := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title :      item.Title,
			Url:         item.Link,
			Description: sql.NullString{String: item.Description, Valid: item.Description != "",},
			PublishedAt: sql.NullTime{Time: publishedAt, Valid: !publishedAt.IsZero()},
			FeedID:      feed.ID,
		}

		_, err = s.db.CreatePost(ctx, params)
		if err != nil {
			// Check if it's a duplicate error
			if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
				// This is a duplicate, just ignore and continue
				fmt.Printf("Skipping duplicate post: %s\n", item.Title)
				continue
			}
			return fmt.Errorf("Error saving RSS post: %w", err)
		}
	}
	return nil
}

func main() {

	config, err := config.Read()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", config.DbUrl)
	if err != nil {
		fmt.Printf("Error opening database connection: %v", err)
		os.Exit(1)
	}
	defer db.Close()
	dbQueries := database.New(db)

	state := State{cfg: &config, db: dbQueries}

	commands := initCommands()

	args := os.Args 
	if len(args) < 2 {
		fmt.Println("You must provide more command line arguments.")
		os.Exit(1)
	}

	cmd_name := args[1]
	cmd_args := args[2:]
	cmd := Command{Name: cmd_name, Args: cmd_args}

	err = commands.run(&state, cmd)
	if err != nil {
		fmt.Println("Error running command", cmd.Name,":", err)
		os.Exit(1)
	}
}