package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/schema"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	BotUserID string
}

const BotUserName = "twilio"
const BotDisplayName = "Twilio"
const BotDescription = "Parses images sent from twilio"

func (p *Plugin) OnActivate() error {
	userID, err := p.Helpers.EnsureBot(
		&model.Bot{
			Username:    BotUserName,
			DisplayName: BotDisplayName,
			Description: BotDescription,
		},
		plugin.ProfileImagePath("assets/profile.png"),
	)
	if err != nil {
		return err
	}

	p.BotUserID = userID
	return nil
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	var e = func(err error) {
		p.API.LogError(err.Error())
		fmt.Fprint(w, "Whoops looks like Michael's code messed up.")
	}

	err := r.ParseForm()
	if err != nil {
		e(err)
		return
	}

	body, err := ParseTwilioBody(r.Form)
	if err != nil {
		e(err)
		return
	}

	resp := fmt.Sprintf("New text from Twilio!\nFrom: %s\n To: %s\n", body.From, body.To)

	team, appErr := p.API.GetTeamByName("test")
	if appErr != nil {
		e(appErr)
		return
	}
	channel, appErr := p.API.GetChannelByName(team.Id, "twilio", false)
	if appErr != nil {
		e(appErr)
		return
	}

	post := &model.Post{
		ChannelId: channel.Id,
		UserId:    p.BotUserID,
	}

	if body.MediaUrl0 != "" {
		b, err := fetchImage(body.MediaUrl0)
		if err != nil {
			e(err)
			return
		}

		fInfo, appErr := p.API.UploadFile(b, channel.Id, "twilio-picture."+strings.Split(body.MediaContentType0, "/")[1])
		if appErr != nil {
			e(appErr)
			return
		}
		post.FileIds = append(post.FileIds, fInfo.Id)
	} else {
		resp += "No picture attached"
	}

	post.Message = resp

	_, appErr = p.API.CreatePost(post)
	if appErr != nil {
		e(appErr)
		return
	}

	fmt.Fprint(w, "Success")
}

func fetchImage(u string) ([]byte, error) {
	client := http.Client{}
	reqImg, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer reqImg.Body.Close()
	b, err := ioutil.ReadAll(reqImg.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ParseTwilioBody(values url.Values) (*TwilioRequest, error) {
	body := new(TwilioRequest)
	decoder := schema.NewDecoder()
	err := decoder.Decode(body, values)
	if err != nil {
		return nil, err
	}

	return body, nil
}

type TwilioRequest struct {
	ToCountry         string //=US
	MediaContentType0 string //=image%2Fjpeg
	ToState           string //=AL
	SmsMessageSid     string //=MMe0e5b5a7407251f7ecf9480c951fbd90
	NumMedia          string //=1
	ToCity            string //=HUNTSVILLE
	FromZip           string //=46202
	SmsSid            string //=MMe0e5b5a7407251f7ecf9480c951fbd90
	FromState         string //=IN
	SmsStatus         string //=received
	FromCity          string //=INDIANAPOLIS
	Body              string //=Fwd%3A%0AFrom%3A13174138408%0ASent%3AWed%2C+Apr+29+20++6%3A41pm%0AMsg%3A
	FromCountry       string //=US
	To                string //=%2B12057298311
	ToZip             string //=35816
	NumSegments       string //=1
	MessageSid        string //=MMe0e5b5a7407251f7ecf9480c951fbd90
	AccountSid        string //=AC434a8ad7c53cbdfac0e118125914dbab
	From              string //=%2B13173621512
	MediaUrl0         string //=https%3A%2F%2Fapi.twilio.com%2F2010-04-01%2FAccounts%2FAC434a8ad7c53cbdfac0e118125914dbab%2FMessages%2FMMe0e5b5a7407251f7ecf9480c951fbd90%2FMedia%2FME7da7e2ff0b4bbd85d11ab88041320a8a
	ApiVersion        string //=2010-04-01
}
