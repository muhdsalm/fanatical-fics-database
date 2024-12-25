package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"time"

	"fmt"
	"html/template"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getConnectionString() string {
	content, err := ioutil.ReadFile("./.secrets.json")
	if err != nil {
		log.Println("Error when opening file, ", err)
	}

	var payload map[string]string
	err = json.Unmarshal(content, &payload)
	if err != nil {
		fmt.Errorf("Error during unmarshal: ", err)
	}

	return payload["connection-string"]
}

type FanaticalDB struct {
	client             *mongo.Client
	database           *mongo.Database
	episodesCollection *mongo.Collection
}

func NewFanaticalDB() FanaticalDB {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(getConnectionString()).SetServerAPIOptions(serverAPI).SetConnectTimeout(10 * time.Second)

	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		log.Fatal(err)
	}

	database := client.Database("fanatical-fics")
	episodesCollection := database.Collection("episodes")
	return FanaticalDB{client: client, database: database, episodesCollection: episodesCollection}
}

func (f FanaticalDB) GetEpisodeByID() {}

type EpisodeSummary struct {
	EpisodeTitle   string             `json:"episodetitle"`
	EpisodeSummary string             `json:"episodesummary"`
	EpisodeNumber  int                `json:"episodenumber"`
	RawId          primitive.ObjectID `bson:"_id" json:"id,omitempty"`
	SearchTerm     string
}

func (f FanaticalDB) GetEpisodesList() []EpisodeSummary {
	cursor, err := f.episodesCollection.Find(context.TODO(), bson.D{{}})
	if err != nil {
		log.Println("Could not read database:", err)
	}
	var elements []EpisodeSummary
	if err = cursor.All(context.TODO(), &elements); err != nil {
		log.Panic(err)
	}
	cursor.Close(context.TODO())
	return elements
}

type Prediction struct {
	Prediction string `json:"prediction"`
	Correct    bool   `json:"correct"`
}

type Clues struct {
	Title      string `json:"title"`
	Genre      string `json:"genre"`
	TimePeriod string `json:"timePeriod"`
}

type Segment struct {
	Title       string       `json:"title"`
	Clues       Clues        `json:"clues"`
	Predictions []Prediction `json:"predictions"`
	Notes       []string     `json:"notes"`
}

type Episode struct {
	EpisodeTitle  string    `json:"episodetitle"`
	EpisodeNumber int       `json:"episodenumber"`
	Warning       string    `json:"warning"`
	Segments      []Segment `json:"segments"`
}

func (f FanaticalDB) GetEpisode(id string) (Episode, error) {
	epId, err := primitive.ObjectIDFromHex(id)
	var epNo int
	if err != nil {
		epNo, err = strconv.Atoi(id)
		if err != nil {
			return Episode{}, err
		}

	}
	var episode Episode

	if epNo == 0 {
		f.episodesCollection.FindOne(context.TODO(), bson.D{{"_id", epId}}).Decode(&episode)
	} else {
		f.episodesCollection.FindOne(context.TODO(), bson.D{{"episodenumber", epNo}}).Decode(&episode)
	}

	if episode.EpisodeTitle == "" {
		return Episode{}, errors.New("This episode does not exist")
	}

	return episode, nil
}

func (f FanaticalDB) searchEpisode(searchTerm string) ([]EpisodeSummary, error) {
	var episodeList []EpisodeSummary
	// cursor, err := f.episodesCollection.Find(context.TODO(), bson.D{{"segments.notes", primitive.Regex{Pattern: searchTerm, Options: "i"}}})
	cursor, err := f.episodesCollection.Find(context.TODO(), bson.D{{"$or", bson.A{
		bson.D{{"segments.notes", primitive.Regex{Pattern: searchTerm, Options: "i"}}},
		bson.D{{"episodetitle", primitive.Regex{Pattern: searchTerm, Options: "i"}}},
	}}})
	if err != nil {
		return []EpisodeSummary{}, err
	}
	cursor.All(context.TODO(), &episodeList)
	return episodeList, nil
}

func (f FanaticalDB) Close() error {
	err := f.client.Disconnect(context.TODO())
	return err
}

func HTMLGetElementsList(w http.ResponseWriter, r *http.Request) {
	var frontpageEpisodeElementTemplate = "templates/frontpageEpisodeElementTemplate.html"
	objectIDParseFunc := template.FuncMap{
		"oid": func(o primitive.ObjectID) string { return o.Hex() },
	}
	tmpl, err := template.New("frontpageEpisodeElementTemplate.html").Funcs(objectIDParseFunc).ParseFiles(frontpageEpisodeElementTemplate)
	if err != nil {
		log.Println("Cannot access front page episode template file:", err)
	}
	if err != nil {
		fmt.Fprint(w, "")
		return
	}
	tenEpisodes := fanaticaldb.GetEpisodesList()
	err = tmpl.Execute(w, tenEpisodes)
	if err != nil {
		log.Println(err)
	}
}

func HTMLGetElement(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[9:]

	episodePageTemplate := "templates/episode.html"
	tmpl, err := template.New("episode.html").ParseFiles(episodePageTemplate)
	if err != nil {
		log.Println("Cannot access episode page template: ", err)
	}
	elementData, err := fanaticaldb.GetEpisode(id)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
	err = tmpl.Execute(w, elementData)
	if err != nil {
		log.Println(err)
	}
}

type Search struct {
	Result     []EpisodeSummary
	SearchTerm string
}

func HTMLSearchElement(w http.ResponseWriter, r *http.Request) {
	var frontpageEpisodeElementTemplate = "templates/frontpageSearchTemplate.html"
	searchTerm := r.URL.Query()["q"][0]
	objectIDParseFunc := template.FuncMap{
		"oid": func(o primitive.ObjectID) string { return o.Hex() },
	}
	tmpl, err := template.New("frontpageSearchTemplate.html").Funcs(objectIDParseFunc).ParseFiles(frontpageEpisodeElementTemplate)
	if err != nil {
		log.Println("Cannot access front page episode template file:", err)
	}
	if err != nil {
		fmt.Fprint(w, "")
		return
	}
	tenEpisodes, err := fanaticaldb.searchEpisode(searchTerm)
	if err != nil {
	}
	err = tmpl.Execute(w, Search{SearchTerm: searchTerm, Result: tenEpisodes})
	if err != nil {
		log.Println(err)
	}
}

var fanaticaldb FanaticalDB

func main() {
	log.Println("Starting the server...")
	fanaticaldb = NewFanaticalDB()
	http.HandleFunc("/episode-list", HTMLGetElementsList)
	http.HandleFunc("/episode/", HTMLGetElement)
	http.HandleFunc("/search", HTMLSearchElement)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})
	log.Fatal(http.ListenAndServe(":3030", nil))

}
