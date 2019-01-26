package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
)

var (
	tokenDir = "auth-tokens"
)

var cfgFile string
var configDir string
var credsFile string
var accessTokensDir string
var googleConfig *oauth2.Config

var rootCmd = &cobra.Command{
	Use: "main",
	Run: func(cmd *cobra.Command, args []string) {
		settings := viper.AllSettings()
		authTokens, ok := settings["auth-tokens"].([]interface{})
		if !ok {
			log.Fatalf("auth-tokens value is not a string list, got %q", settings["auth-tokens"])
		}
		var events []*calendar.Event
		for _, tokenPathIf := range authTokens {
			tokenPath, ok := tokenPathIf.(string)
			if !ok {
				log.Fatalf("auth-tokens value is not a string, got %q", tokenPathIf)
			}
			eventsRet, err := getNextCalendarItems(tokenPath)
			if err != nil {
				log.Fatal(err)
			}
			events = append(events, eventsRet...)
		}
		if len(events) == 0 {
			fmt.Println("No upcoming events found.")
		} else {
			for _, item := range events {
				date := item.Start.DateTime
				if date == "" {
					date = item.Start.Date
				}
				fmt.Printf("%v (%v)\n", item.Summary, date)
			}
		}
	},
}

func getNextCalendarItems(tokenPath string) ([]*calendar.Event, error) {
	client, err := getClient(googleConfig, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to get client: %v", err)
	}
	srv, err := calendar.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Calendar client: %v", err)
	}

	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve next ten of the user's events: %v", err)
	}
	fmt.Println("Upcoming events:")
	return events.Items, nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Saves a token to a file path.
func saveToken(token *oauth2.Token) {
	ls, err := os.Stat(accessTokensDir)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(accessTokensDir, 0700); err != nil {
			log.Fatal(err)
		}
	} else if !ls.IsDir() {
		log.Fatalf("config path is not a directory, was %q", accessTokensDir)
	}
	u := uuid.Must(uuid.NewV4())
	path := path.Join(accessTokensDir, u.String())
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
	authTokens := append(viper.GetStringSlice("auth-tokens"), path)
	viper.Set("auth-tokens", authTokens)
	err = viper.WriteConfig()
	if err != nil {
		log.Fatalf("Unable to write config: %v", err)
	}
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config, tokFile string) (*http.Client, error) {
	f, err := os.Open(tokFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	if err != nil {
		return nil, err
	}
	return config.Client(context.Background(), tok), nil
}

var registerCmd = &cobra.Command{
	Use: "register",
	Run: func(cmd *cobra.Command, args []string) {
		authToken := getTokenFromWeb(googleConfig)
		saveToken(authToken)
		fmt.Println("register", configDir)
	},
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	configDir = os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		// Find home directory.
		var err error
		configDir, err = homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		configDir = path.Join(configDir, ".config")
	}
	configDir = path.Join(configDir, "i3block-gcalendar")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(configDir)
		ls, err := os.Stat(configDir)
		if os.IsNotExist(err) {
			if err = os.MkdirAll(configDir, 0700); err != nil {
				log.Fatal(err)
			}
		} else if !ls.IsDir() {
			log.Fatalf("config path is not a directory, was %q", configDir)
		}

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.SetConfigName("config")
	}
	viper.AutomaticEnv()

	//TODO create config if not existent
	//	if err := viper.WriteConfig(); err != nil {
	//		log.Fatalf("Could not write config: %v", err)
	//	}
	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Unable to read read config file: %v", err)
	}
	if accessTokensDir == "" {
		tokensDir := viper.GetString("access-tokens")
		if tokensDir != "" {
			accessTokensDir = tokensDir
		} else {
			accessTokensDir = path.Join(configDir, tokenDir)
		}
	}

	b, err := ioutil.ReadFile(path.Join(configDir, credsFile))
	if err != nil {
		log.Fatalf("Unable to read application secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	googleConfig, err = google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
}

// Register calendar through CLI

// Get all events from all calendar
// Sort
// Display first with entry with time and remaining hours

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		// Do exit stuff
		<-done
		log.Fatal("Exiting")
	}()
	go func() {
		sig := <-sigs
		log.WithFields(log.Fields{
			"signal": sig,
		}).Warning("Signal caught")
		done <- true
	}()

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default is $XDG_CONFIG_HOME/i3block-gcalendar/config)")
	rootCmd.PersistentFlags().StringVar(&accessTokensDir, "access-tokens", "", "access-tokens directory path (default is $XDG_CONFIG_HOME/i3block-gcalendar/access-tokens)")
	rootCmd.PersistentFlags().StringVar(&credsFile, "creds", "credentials.json", "credentials file path (default is CONFIG/credentials.json)")
	rootCmd.AddCommand(registerCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
