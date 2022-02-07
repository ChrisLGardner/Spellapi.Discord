package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	"gopkg.in/yaml.v3"
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

	if !strings.HasPrefix(m.Content, "?") {
		return
	}

	ctx := context.Background()
	var span *trace.Span
	me := hnydiscordgo.MessageEvent{Message: m.Message, Context: ctx}

	ctx, span = hnydiscordgo.StartSpanOrTraceFromMessage(&me, s)
	defer span.Send()

	m.Content = strings.Replace(m.Content, "?", "", 1)
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
		?spell <Spell Name> - Finds the spell specified if possible.
			when there are multiple spells matching you can narrow it down
			using filters like "system=dnd" or "level: 2"
		?spell add - Will add a new spell, either using the content of the message or via attachment.
			See https://github.com/ChrisLGardner/Spellapi.Discord for the format to be used.
		`
		sendResponse(ctx, s, m.ChannelID, help)
	} else if command == "spell" {
		beeline.AddField(ctx, "command", "spell")

		if strings.HasPrefix(m.Content, "add") {
			m.Content = strings.TrimPrefix(strings.TrimPrefix(m.Content, "add"), "\n")
			resp, err := createSpell(ctx, m.Message)
			if err != nil {
				beeline.AddField(ctx, "error", err)
				sendResponse(ctx, s, m.ChannelID, "Spell not found")
			}
			sendResponse(ctx, s, m.ChannelID, resp)
		} else {
			resp, err := getSpell(ctx, m.Content)
			if err != nil {
				beeline.AddField(ctx, "error", err)
				sendResponse(ctx, s, m.ChannelID, "Spell not found")
			}

			sendSpell(ctx, s, m.ChannelID, resp)
		}
	} else if command == "metadata" {
		beeline.AddField(ctx, "command", "metadata")

		if strings.TrimSpace(m.Content) != "metadata" {
			resp, err := getSpellMetadataValues(ctx, m.Message)
			if err != nil {
				beeline.AddField(ctx, "error", err)
				sendResponse(ctx, s, m.ChannelID, err.Error())
			}
			sendResponse(ctx, s, m.ChannelID, resp)
		} else {
			resp, err := getSpellMetadataNames(ctx, m.Message)
			if err != nil {
				beeline.AddField(ctx, "error", err)
				sendResponse(ctx, s, m.ChannelID, err.Error())
			}
			sendResponse(ctx, s, m.ChannelID, resp)
		}
	}
}

func sendResponse(ctx context.Context, s *discordgo.Session, cid string, m string) {

	ctx, span := beeline.StartSpan(ctx, "sendResponse")
	defer span.Send()
	beeline.AddField(ctx, "response", m)
	beeline.AddField(ctx, "channel", cid)

	s.ChannelMessageSend(cid, m)

}

func sendSpell(ctx context.Context, s *discordgo.Session, cid string, spell []Spell) {

	ctx, span := beeline.StartSpan(ctx, "sendSpell")
	defer span.Send()
	beeline.AddField(ctx, "spell", spell)
	beeline.AddField(ctx, "channel", cid)
	if len(spell) < 4 {
		for _, indivSpell := range spell {
			spellEmbed := formatSpellEmbed(ctx, indivSpell)

			s.ChannelMessageSendEmbed(cid, spellEmbed)
		}
	} else {
		var sb strings.Builder
		sb.WriteString("**Spell Name** | **System**\n")
		sb.WriteString("```")
		for _, indivSpell := range spell {
			sb.WriteString(fmt.Sprintf("%s | %s\n", indivSpell.Name, indivSpell.Metadata.System))
		}
		sb.WriteString("```")
		s.ChannelMessageSend(cid, sb.String())
	}

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

type SpellResponse struct {
	Spell []Spell
}

func (sr *SpellResponse) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("no bytes to unmarshal")
	}
	// See if we can guess based on the first character
	switch b[0] {
	case '{':
		return sr.unmarshalSingle(b)
	case '[':
		return sr.unmarshalMany(b)
	}
	// TODO: Figure out what do we do here
	return nil

}

func (sr *SpellResponse) unmarshalSingle(b []byte) error {
	var s Spell

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	sr.Spell = []Spell{s}
	return nil
}

func (sr *SpellResponse) unmarshalMany(b []byte) error {
	var s []Spell

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	sr.Spell = s
	return nil
}

func getSpell(ctx context.Context, s string) ([]Spell, error) {

	ctx, span := beeline.StartSpan(ctx, "getSpell")
	defer span.Send()

	getUrl := formatGetUrl(ctx, s)

	client := &http.Client{
		Transport: hnynethttp.WrapRoundTripper(http.DefaultTransport),
		Timeout:   time.Second * 5,
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", getUrl, nil)
	resp, err := client.Do(req)
	if err != nil {
		beeline.AddField(ctx, "error", err)
		return []Spell{}, err
	}
	defer resp.Body.Close()

	var spellResponse SpellResponse

	err = json.NewDecoder(resp.Body).Decode(&spellResponse)
	if err != nil {

		beeline.AddField(ctx, "error", err)
		return []Spell{}, err
	}

	beeline.AddField(ctx, "response", spellResponse.Spell)

	return spellResponse.Spell, nil
}

func formatGetUrl(ctx context.Context, s string) string {
	ctx, span := beeline.StartSpan(ctx, "formatGetUrl")
	defer span.Send()

	uri := fmt.Sprintf("%s/spells", apiUrl)

	s, queryParameters := parseQuery(ctx, s)

	if s != "" {
		uri = fmt.Sprintf("%s/%s", uri, s)
	}

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

func createSpell(ctx context.Context, message *discordgo.Message) (string, error) {

	ctx, span := beeline.StartSpan(ctx, "createSpell")
	defer span.Send()

	var spellRaw []byte
	client := &http.Client{
		Transport: hnynethttp.WrapRoundTripper(http.DefaultTransport),
		Timeout:   time.Second * 20,
	}

	if len(message.Attachments) > 0 {

		beeline.AddField(ctx, "createSpell.Attachment.Url", message.Attachments[0].URL)
		beeline.AddField(ctx, "createSpell.Attachment.Filename", message.Attachments[0].Filename)

		getReq, _ := http.NewRequestWithContext(ctx, "GET", message.Attachments[0].URL, nil)
		resp, err := client.Do(getReq)
		if err != nil {
			beeline.AddField(ctx, "createSpell.Error", err)
			return "", err
		}
		defer resp.Body.Close()

		spellRaw, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			beeline.AddField(ctx, "createSpell.Attachment.Error", err)
			return "", err
		}

	} else {
		spellRaw = []byte(message.Content)
	}

	if spellRaw[0] != '{' {
		temp := make(map[string]interface{})

		err := yaml.Unmarshal(spellRaw, &temp)
		if err != nil {
			beeline.AddField(ctx, "createSpell.RawToYaml.Error", err)
			return "", err
		}

		spellRaw, err = json.Marshal(temp)
		if err != nil {
			beeline.AddField(ctx, "createSpell.YamlToJson.Error", err)
			return "", err
		}
	}

	postUrl := fmt.Sprintf("%s/spells", apiUrl)
	spell := bytes.NewReader(spellRaw)

	postReq, _ := http.NewRequestWithContext(ctx, "POST", postUrl, spell)
	postResp, err := client.Do(postReq)
	if err != nil {
		beeline.AddField(ctx, "createSpell.Error", err)
		return "", err
	}
	defer postResp.Body.Close()

	parsedResp, err := io.ReadAll(postResp.Body)
	if err != nil {
		beeline.AddField(ctx, "createSpell.Error", err)
		return "", err
	}

	return string(parsedResp), nil
}

func getSpellMetadataNames(ctx context.Context, m *discordgo.Message) (string, error) {
	ctx, span := beeline.StartSpan(ctx, "getSpellMetadataNames")
	defer span.Send()

	client := &http.Client{
		Transport: hnynethttp.WrapRoundTripper(http.DefaultTransport),
		Timeout:   time.Second * 20,
	}
	Url := fmt.Sprintf("%s/spellmetadata", apiUrl)

	getReq, _ := http.NewRequestWithContext(ctx, "GET", Url, nil)
	getReq.Header = map[string][]string{
		"X-SPELLAPI-USERID": {m.Author.ID},
	}

	resp, err := client.Do(getReq)
	if err != nil {
		beeline.AddField(ctx, "getSpellMetadataNames.Error", err)
		return "", err
	}
	defer resp.Body.Close()

	metadataRaw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		beeline.AddField(ctx, "getSpellMetadataNames.Response.Error", err)
		return "", err
	}

	beeline.AddField(ctx, "getSpellMetadataNames.Response.Raw", metadataRaw)
	var arr []string
	_ = json.Unmarshal(metadataRaw, &arr)

	var metadata strings.Builder
	metadata.WriteString("Found the following possible metadata names:\n")
	for _, v := range arr {
		metadata.WriteString(fmt.Sprintln(v))
	}

	return metadata.String(), nil

}

func getSpellMetadataValues(ctx context.Context, m *discordgo.Message) (string, error) {
	ctx, span := beeline.StartSpan(ctx, "getSpellMetadataValues")
	defer span.Send()

	client := &http.Client{
		Transport: hnynethttp.WrapRoundTripper(http.DefaultTransport),
		Timeout:   time.Second * 20,
	}
	Url := fmt.Sprintf("%s/spellmetadata/%s", apiUrl, m.Content)
	beeline.AddField(ctx, "getSpellMetadataValues.Url", Url)

	getReq, _ := http.NewRequestWithContext(ctx, "GET", Url, nil)
	getReq.Header = map[string][]string{
		"X-SPELLAPI-USERID": {m.Author.ID},
	}

	resp, err := client.Do(getReq)
	if err != nil {
		beeline.AddField(ctx, "getSpellMetadataValues.Error", err)
		return "", err
	}
	defer resp.Body.Close()

	metadataRaw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		beeline.AddField(ctx, "getSpellMetadataValues.Response.Error", err)
		return "", err
	}

	beeline.AddField(ctx, "getSpellMetadataValues.Response.Raw", metadataRaw)
	arr := make(map[string][]string)
	_ = json.Unmarshal(metadataRaw, &arr)

	var metadata strings.Builder
	metadata.WriteString(fmt.Sprintf("Found the following possible metadata values for %s:\n", m.Content))
	for _, v := range arr[m.Content] {
		metadata.WriteString(fmt.Sprintln(v))
	}

	return metadata.String(), nil
}
