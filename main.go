package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/ChrisLGardner/Spellapi.Discord/hnydiscordgo"
	"github.com/bwmarrin/discordgo"
	"github.com/honeycombio/beeline-go"
	"github.com/honeycombio/beeline-go/trace"
	"github.com/honeycombio/beeline-go/wrappers/hnynethttp"
)

var apiUrl string

func main() {

	beeline.Init(beeline.Config{
		WriteKey: os.Getenv("HONEYCOMB_KEY"),
		Dataset:  os.Getenv("HONEYCOMB_DATASET"),
	})

	defer beeline.Close()

	// Open a simple Discord session
	token := os.Getenv("DISCORD_TOKEN")
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		panic(err)
	}
	err = session.Open()
	if err != nil {
		panic(err)
	}

	// Wait for the user to cancel the process
	defer func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
		<-sc
	}()

	apiUrl = os.Getenv("API_URL")
	session.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	session.AddHandler(MessageRespond)
}

//MessageRespond is the handler for which message respond function should be called
func MessageRespond(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if !strings.HasPrefix(m.Content, "!") {
		return
	}

	ctx := context.Background()
	var span *trace.Span
	me := hnydiscordgo.MessageEvent{Message: m.Message, Context: ctx}

	ctx, span = hnydiscordgo.StartSpanOrTraceFromMessage(&me, s)

	m.Content = strings.Replace(m.Content, "!", "", 1)
	span.AddField("name", "MessageRespond")

	split := strings.SplitAfterN(m.Content, " ", 2)
	command := strings.Trim(strings.ToLower(split[0]), " ")
	if len(split) == 2 {
		m.Content = split[1]
	}

	beeline.AddField(ctx, "parsedCommand", command)
	beeline.AddField(ctx, "remainingContent", m.Content)

	if command == "help" {
		beeline.AddField(ctx, "command", "help")
		help := `Commands available:
		`
		sendResponse(ctx, s, m.ChannelID, help)
	} else if command == "spell" {
		beeline.AddField(ctx, "command", "help")

		resp, err := getSpell(ctx, m.Content)
		if err != nil {
			beeline.AddField(ctx, "error", err)
			sendResponse(ctx, s, m.ChannelID, "Spell not found")
		}

		sendSpell(ctx, s, m.ChannelID, resp)
	}
}

func sendResponse(ctx context.Context, s *discordgo.Session, cid string, m string) {

	ctx, span := beeline.StartSpan(ctx, "sendResponse")
	defer span.Send()
	beeline.AddField(ctx, "response", m)
	beeline.AddField(ctx, "channel", cid)

	s.ChannelMessageSend(cid, m)

}

func sendSpell(ctx context.Context, s *discordgo.Session, cid string, spell Spell) {

	ctx, span := beeline.StartSpan(ctx, "sendSpell")
	defer span.Send()
	beeline.AddField(ctx, "spell", spell)
	beeline.AddField(ctx, "channel", cid)

	spellEmbed := formatSpellEmbed(ctx, spell)

	s.ChannelMessageSendEmbed(cid, spellEmbed)

}

type Spell struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	SpellData   map[string]interface{} `json:"spelldata,omitempty"`
	Metadata    SpellMetadata          `json:"metadata,omitempty"`
}
type SpellMetadata struct {
	System string `json:"system" bson:"system"`
}

func getSpell(ctx context.Context, s string) (Spell, error) {

	ctx, span := beeline.StartSpan(ctx, "getSpell")
	defer span.Send()

	getUrl := formatGetUrl(ctx, s)

	client := &http.Client{
		Transport: hnynethttp.WrapRoundTripper(http.DefaultTransport),
		Timeout:   time.Second * 5,
	}
	req, _ := http.NewRequest("GET", getUrl, nil)
	resp, err := client.Do(req)
	if err != nil {
		beeline.AddField(ctx, "error", err)
		return Spell{}, err
	}
	defer resp.Body.Close()

	var spell Spell

	err = json.NewDecoder(resp.Body).Decode(&spell)
	if err != nil {
		beeline.AddField(ctx, "error", err)
		return Spell{}, err
	}

	beeline.AddField(ctx, "response", spell)

	return spell, nil
}

func formatGetUrl(ctx context.Context, s string) string {
	ctx, span := beeline.StartSpan(ctx, "formatGetUrl")
	defer span.Send()

	uri := fmt.Sprintf("%s/spells/", apiUrl)

	s, queryParameters := parseQuery(ctx, s)

	uri = fmt.Sprintf("%s%s", uri, s)

	params := url.Values{}
	for k, v := range queryParameters {
		params.Add(k, v)
	}

	if len(params) != 0 {
		uri = fmt.Sprintf("%s?%s", uri, params.Encode())
	}

	return uri
}

func parseQuery(ctx context.Context, query string) (string, map[string]string) {

	ctx, span := beeline.StartSpan(ctx, "parseQuery")
	defer span.Send()

	pattern := regexp.MustCompile(`((?P<query>\w+)[=:] ?(?P<value>\w+))+`)
	names := pattern.SubexpNames()
	elements := map[string]string{}

	matchingStrings := pattern.FindAllStringSubmatch(query, -1)

	beeline.AddField(ctx, "parseQuery.Matcheslength", len(matchingStrings))
	beeline.AddField(ctx, "parseQuery.Matches", matchingStrings)

	for _, match := range matchingStrings {

		for i, n := range names {

			if n == "query" {
				elements[match[i]] = match[i+1]
			}
		}
	}

	remainingCommand := strings.TrimSpace(pattern.ReplaceAllString(query, ""))

	beeline.AddField(ctx, "parseQuery.ParsedMatches", elements)
	beeline.AddField(ctx, "parseQuery.RemainingString", remainingCommand)

	return remainingCommand, elements
}

func formatSpellEmbed(ctx context.Context, spell Spell) *discordgo.MessageEmbed {
	ctx, span := beeline.StartSpan(ctx, "formatSpellEmbed")
	defer span.Send()

	beeline.AddField(ctx, "formatSpellEmbed.Spell", spell)

	embedFields := []*discordgo.MessageEmbedField{}
	for k, v := range spell.SpellData {
		field := &discordgo.MessageEmbedField{
			Name:   strings.Title(k),
			Value:  fmt.Sprintf("%v", v),
			Inline: true,
		}
		embedFields = append(embedFields, field)
	}

	spellEmbed := discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       spell.Name,
		Description: spell.Description,
		Fields:      embedFields,
	}

	return &spellEmbed
}
