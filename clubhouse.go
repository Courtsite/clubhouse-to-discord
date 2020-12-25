package function

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type ClubhouseApiClient struct {
	ApiToken string
}

// https://clubhouse.io/api/rest/v3/#Get-Member
type GetMemberResponse struct {
	CreatedAt  time.Time `json:"created_at"`
	Disabled   bool      `json:"disabled"`
	EntityType string    `json:"entity_type"`
	GroupIds   []string  `json:"group_ids"`
	ID         string    `json:"id"`
	Profile    struct {
		Deactivated bool `json:"deactivated"`
		DisplayIcon struct {
			CreatedAt  time.Time `json:"created_at"`
			EntityType string    `json:"entity_type"`
			ID         string    `json:"id"`
			UpdatedAt  time.Time `json:"updated_at"`
			URL        string    `json:"url"`
		} `json:"display_icon"`
		EmailAddress           string `json:"email_address"`
		EntityType             string `json:"entity_type"`
		GravatarHash           string `json:"gravatar_hash"`
		ID                     string `json:"id"`
		MentionName            string `json:"mention_name"`
		Name                   string `json:"name"`
		TwoFactorAuthActivated bool   `json:"two_factor_auth_activated"`
	} `json:"profile"`
	Role      string    `json:"role"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *ClubhouseApiClient) GetMember(memberPublicID string) (*GetMemberResponse, error) {
	httpClient := http.Client{}

	apiURL := fmt.Sprintf("https://api.clubhouse.io/api/v3/members/%s", memberPublicID)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Clubhouse-Token", c.ApiToken)

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to get member: %q (status code: %d)", data, res.StatusCode)
	}

	var memberRes GetMemberResponse
	err = json.Unmarshal(data, &memberRes)
	if err != nil {
		log.Printf("\nraw data received: %q \n", data)
		return nil, err
	}

	return &memberRes, nil
}
