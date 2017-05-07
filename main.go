// ...coming soon...
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"

	"github.com/dustin/go-humanize"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

const filesClientSecret = "client_secret.json"
const filesToken = "token.json"

type message struct {
	id   string
	size int64
}

type bySize []message

func (items bySize) Len() int {
	return len(items)
}

func (items bySize) Swap(one int, two int) {
	items[one], items[two] = items[two], items[one]
}

func (items bySize) Less(one int, two int) bool {
	return items[one].size > items[two].size
}

func getClient(background context.Context, config *oauth2.Config) *http.Client {
	token := getToken(config)
	return config.Client(background, token)
}

func getConfig() *oauth2.Config {
	bytes, bytesErr := ioutil.ReadFile(filesClientSecret)
	if bytesErr != nil {
		log.Fatalf("%v\n", bytesErr)
	}

	config, configErr := google.ConfigFromJSON(bytes, gmail.MailGoogleComScope)
	if configErr != nil {
		log.Fatalf("%v\n", configErr)
	}

	return config
}

func getService(client *http.Client) *gmail.Service {
	service, serviceErr := gmail.New(client)
	if serviceErr != nil {
		log.Fatalf("%v\n", serviceErr)
	}

	return service
}

func getToken(config *oauth2.Config) *oauth2.Token {
	token, err := getTokenFromFile()
	if err == nil {
		return token
	}

	token = getTokenFromGoogle(config)
	setToken(token)
	return token
}

func getTokenFromFile() (*oauth2.Token, error) {
	file, fileErr := os.Open(filesToken)
	if fileErr != nil {
		return nil, fileErr
	}
	defer file.Close()

	token := &oauth2.Token{}
	err := json.NewDecoder(file).Decode(token)
	return token, err
}

func getTokenFromGoogle(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	log.Println("Open the following URL in your browser:")
	log.Println(authURL)
	log.Printf("Code:")
	log.Printf(" ")
	code := ""
	_, scanErr := fmt.Scan(&code)
	if scanErr != nil {
		log.Fatalf("%v\n", scanErr)
	}

	token, tokenErr := config.Exchange(oauth2.NoContext, code)
	if tokenErr != nil {
		log.Fatalf("%v\n", tokenErr)
	}

	return token
}

func setToken(token *oauth2.Token) {
	file, fileErr := os.OpenFile(filesToken, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if fileErr != nil {
		log.Fatalf("%v\n", fileErr)
	}
	defer file.Close()

	json.NewEncoder(file).Encode(token)
}

func fetch(q string, maxResults int64) []message {
	messages := []message{}
	background := context.Background()
	config := getConfig()
	client := getClient(background, config)
	service := getService(client)
	pageToken := ""
	for {
		request := service.Users.Messages.List("me").Q(q).MaxResults(maxResults)
		if pageToken != "" {
			request.PageToken(pageToken)
		}
		response, responseErr := request.Do()
		if responseErr != nil {
			log.Fatalf("%v\n", responseErr)
		}
		length := len(response.Messages)
		log.Printf("Fetching messages: %3d\n", length)
		for _, value := range response.Messages {
			do, doErr := service.Users.Messages.Get("me", value.Id).Do()
			if doErr != nil {
				log.Fatalf("%v\n", doErr)
			}
			m := message{
				id:   do.Id,
				size: do.SizeEstimate,
			}
			messages = append(messages, m)
		}
		if response.NextPageToken == "" {
			break
		}
		pageToken = response.NextPageToken
	}
	return messages
}

func report(messages []message, limit int) {
	log.Println("")

	totalMessagesInt := len(messages)
	totalMessagesInt64 := int64(totalMessagesInt)
	totalMessagesHumanize := humanize.Comma(totalMessagesInt64)
	log.Printf("Total Messages: %v\n", totalMessagesHumanize)

	totalBytesInt := 0
	totalBytesInt64 := int64(totalBytesInt)
	for _, message := range messages {
		totalBytesInt64 = totalBytesInt64 + message.size
	}
	totalBytesUint64 := uint64(totalBytesInt64)
	totalBytesHumanize := humanize.Bytes(totalBytesUint64)
	log.Printf("Total Size    : %v\n", totalBytesHumanize)

	sort.Sort(bySize(messages))

	if limit > totalMessagesInt {
		limit = totalMessagesInt
	}
	messages = messages[0:limit]

	log.Println("")

	for _, message := range messages {
		bytesUint64 := uint64(message.size)
		bytesHumanize := humanize.Bytes(bytesUint64)
		log.Printf("https://mail.google.com/mail/u/0/#inbox/%s [%9s]\n", message.id, bytesHumanize)
	}
}

func main() {
	q := flag.String("q", "", "Query (https://support.google.com/mail/answer/7190?hl=en)")
	maxResults := flag.Int64("m", 999, "Maximum number of results")
	limit := flag.Int("l", 10, "Limit")
	flag.Parse()
	if *q == "" {
		log.Fatalln("Invalid Query")
	}
	messages := fetch(*q, *maxResults)
	report(messages, *limit)
}
