package function

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Expanded from https://clubhouse.io/api/webhook/v1/#Webhook-Format
type ClubhouseWebhook struct {
	Actions    []ClubhouseAction    `json:"actions"`
	ChangedAt  time.Time            `json:"changed_at"`
	ID         string               `json:"id"`
	MemberID   string               `json:"member_id"`
	PrimaryID  int                  `json:"primary_id"`
	References []ClubhouseReference `json:"references"`
	Paper      *string              `json:"paper,omitempty"`
	Version    string               `json:"version"`
}

type ClubhouseAction struct {
	Action          string           `json:"action"`
	AppURL          string           `json:"app_url"`
	AuthorID        string           `json:"author_id"`
	Changes         ClubhouseChanges `json:"changes"`
	Complete        bool             `json:"complete,omitempty"`
	Description     string           `json:"description"`
	EntityType      string           `json:"entity_type"`
	EpicID          int              `json:"epic_id"`
	Estimate        int              `json:"estimate,omitempty"`
	FollowerIds     []string         `json:"follower_ids"`
	ID              int              `json:"id"`
	IterationID     int              `json:"iteration_id"`
	MilestoneID     int              `json:"milestone_id"`
	Name            string           `json:"name"`
	OwnerIds        []string         `json:"owner_ids"`
	Position        int64            `json:"position"`
	ProjectID       int              `json:"project_id"`
	RequestedByID   string           `json:"requested_by_id"`
	StoryType       string           `json:"story_type"`
	TaskIds         []int            `json:"task_ids,omitempty"`
	Town            *string          `json:"town,omitempty"`
	Text            string           `json:"text"`
	URL             string           `json:"url"`
	WorkflowStateID int              `json:"workflow_state_id"`
}

type ClubhouseReference struct {
	AppURL     string `json:"app_url"`
	EntityType string `json:"entity_type"`
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
}

type ClubhouseChanges struct {
	Archived *struct {
		New bool `json:"new"`
		Old bool `json:"old"`
	} `json:"archived,omitempty"`
	Blocker *struct {
		New bool `json:"new"`
		Old bool `json:"old"`
	} `json:"blocker,omitempty"`
	CommentIds *struct {
		Adds    []int `json:"adds"`
		Removes []int `json:"removes"`
	} `json:"comment_ids,omitempty"`
	Completed *struct {
		New bool `json:"new"`
		Old bool `json:"old"`
	} `json:"completed,omitempty"`
	CompletedAt *struct {
		New time.Time `json:"new"`
	} `json:"completed_at,omitempty"`
	Deadline *struct {
		New *time.Time `json:"new,omitempty"`
		Old *time.Time `json:"old,omitempty"`
	} `json:"deadline,omitempty"`
	EpicID *struct {
		New *int `json:"new,omitempty"`
		Old *int `json:"old,omitempty"`
	} `json:"epic_id,omitempty"`
	Estimate *struct {
		New *int `json:"new,omitempty"`
		Old *int `json:"old,omitempty"`
	} `json:"estimate,omitempty"`
	FollowerIds *struct {
		Adds []string `json:"adds"`
	} `json:"follower_ids,omitempty"`
	IterationID *struct {
		New *int `json:"new,omitempty"`
		Old *int `json:"old,omitempty"`
	} `json:"iteration_id,omitempty"`
	LabelIds *struct {
		Adds    []int `json:"adds"`
		Removes []int `json:"removes"`
	} `json:"label_ids,omitempty"`
	OwnerIds *struct {
		Adds    []string `json:"adds"`
		Removes []string `json:"removes"`
	} `json:"owner_ids,omitempty"`
	Position *struct {
		New int64 `json:"new"`
		Old int64 `json:"old"`
	} `json:"position,omitempty"`
	ProjectID *struct {
		New int `json:"new"`
		Old int `json:"old"`
	} `json:"project_id,omitempty"`
	Started *struct {
		New bool `json:"new"`
		Old bool `json:"old"`
	} `json:"started,omitempty"`
	StartedAt *struct {
		New time.Time `json:"new"`
	} `json:"started_at,omitempty"`
	StoryType *struct {
		New string `json:"new"`
		Old string `json:"old"`
	} `json:"story_type,omitempty"`
	Text *struct {
		New string `json:"new"`
		Old string `json:"old"`
	} `json:"text,omitempty"`
	WorkflowStateID *struct {
		New int `json:"new"`
		Old int `json:"old"`
	} `json:"workflow_state_id,omitempty"`
}

type OverallAction int

const (
	OverallAction_UNKNOWN OverallAction = iota
	OverallAction_Create
	OverallAction_Update
)

type DiscordWebhook struct {
	Content string  `json:"content"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

type Embed struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	Color       int     `json:"color"`
	Fields      []Field `json:"fields,omitempty"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func toDiscord(clubhouseApiClient *ClubhouseApiClient, webhook ClubhouseWebhook) (*DiscordWebhook, error) {
	var webhookTitle string
	var webhookURL string
	var fields []Field
	var colour int

	firstAction := webhook.Actions[0]

	// actionsByID := getActionsByID(webhook)
	referencesByTypeID := getReferencesByTypeID(webhook)

	var err error

	switch firstAction.Action {
	case "create":
		colour = 5424154
		fields = getActionFields(referencesByTypeID, firstAction)

		if len(fields) == 0 {
			return nil, nil
		}
	case "update":
		colour = 16440084
		fields, err = getChangesFields(clubhouseApiClient, referencesByTypeID, firstAction.Changes)
		if err != nil {
			return nil, err
		}

		if len(fields) == 0 {
			return nil, nil
		}
	case "delete":
		colour = 16065069
	default:
		return nil, nil
	}

	if firstAction.Action != "" && firstAction.EntityType != "" && firstAction.Name != "" {
		if webhook.MemberID != "" {
			member, err := clubhouseApiClient.GetMember(webhook.MemberID)
			if err != nil {
				return nil, err
			}

			webhookTitle = fmt.Sprintf(
				"%s %sd %s: %s",
				strings.Title(member.Profile.Name),
				firstAction.Action,
				firstAction.EntityType,
				firstAction.Name,
			)
		} else {
			webhookTitle = fmt.Sprintf(
				"%sd %s: %s",
				strings.Title(firstAction.Action),
				firstAction.EntityType,
				firstAction.Name,
			)
		}
	}
	if firstAction.AppURL != "" {
		webhookURL = firstAction.AppURL
	}

	if webhookTitle == "" || webhookURL == "" {
		return nil, nil
	}

	return &DiscordWebhook{
		Embeds: []Embed{
			{
				Title:  webhookTitle,
				URL:    webhookURL,
				Color:  colour,
				Fields: fields,
			},
		},
	}, nil
}

func F(w http.ResponseWriter, r *http.Request) {
	discordWebhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordWebhookURL == "" {
		log.Fatalln("`DISCORD_WEBHOOK_URL` is not set in the environment")
	}

	if _, err := url.Parse(discordWebhookURL); err != nil {
		log.Fatalln(err)
	}

	clubhouseApiToken := os.Getenv("CLUBHOUSE_API_TOKEN")
	if clubhouseApiToken == "" {
		log.Fatalln("`CLUBHOUSE_API_TOKEN` is not set in the environment")
	}

	clubhouseApiClient := &ClubhouseApiClient{ApiToken: clubhouseApiToken}

	if contentType := r.Header.Get("Content-Type"); r.Method != "POST" || contentType != "application/json" {
		log.Printf("\ninvalid method / content-type: %s / %s \n", r.Method, contentType)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid request"))
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatalln(err)
	}

	clubhouseWebhookSecret := os.Getenv("CLUBHOUSE_WEBHOOK_SECRET")

	if clubhouseSignature := strings.TrimSpace(r.Header.Get("Clubhouse-Signature")); clubhouseSignature != "" {
		if clubhouseWebhookSecret == "" {
			log.Fatalln("received webhook with signature, but `CLUBHOUSE_WEBHOOK_SECRET` was not set in the environment")
		}

		mac := hmac.New(sha256.New, []byte(strings.TrimSpace(clubhouseWebhookSecret)))
		_, err = mac.Write(data)
		if err != nil {
			log.Fatalln(err)
		}
		expectedMAC := mac.Sum(nil)

		clubhouseHexSignature, err := hex.DecodeString(clubhouseSignature)
		if err != nil {
			log.Fatalln(err)
		}

		if !hmac.Equal(clubhouseHexSignature, expectedMAC) {
			log.Printf("\nsignature does not match: %s (got) != %s (want) \n", hex.EncodeToString(clubhouseHexSignature), hex.EncodeToString(expectedMAC))
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid request"))
			return
		}
	}

	var webhook ClubhouseWebhook
	err = json.Unmarshal(data, &webhook)
	if err != nil {
		log.Printf("\nraw data received: %q \n", data)
		log.Fatalln(err)
	}

	if webhook.Version != "v1" {
		log.Println("version not supported:", webhook.Version)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid request"))
		return
	}

	if totalActions := len(webhook.Actions); totalActions != 1 {
		log.Printf("\nunhandled raw data received: %q \n", data)
		w.WriteHeader(http.StatusOK)
		return
	}

	discordWebhook, err := toDiscord(clubhouseApiClient, webhook)
	if err != nil {
		log.Printf("\nraw data received: %q \n", data)
		log.Fatalln(err)
	}
	if discordWebhook == nil {
		log.Printf("\nunhandled raw data received: %q \n", data)
		w.WriteHeader(http.StatusOK)
		return
	}

	payload, err := json.Marshal(discordWebhook)
	if err != nil {
		log.Fatalln(err)
	}

	res, err := http.Post(discordWebhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalln(err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Println("payload", string(payload))
		log.Fatalln("unexpected status code", res.StatusCode)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(discordWebhook)
	if err != nil {
		log.Fatalln(err)
	}
}

func getActionsByID(webhook ClubhouseWebhook) map[string]ClubhouseAction {
	actionsByID := make(map[string]ClubhouseAction)

	for _, action := range webhook.Actions {
		actionsByID[strconv.Itoa(action.ID)] = action
	}

	return actionsByID
}

func getReferencesByTypeID(webhook ClubhouseWebhook) map[string]ClubhouseReference {
	referencesByTypeID := make(map[string]ClubhouseReference)

	for _, reference := range webhook.References {
		typeID := fmt.Sprintf("%s:%d", reference.EntityType, reference.ID)
		referencesByTypeID[typeID] = reference
	}

	return referencesByTypeID
}

func getActionFields(referencesByTypeID map[string]ClubhouseReference, action ClubhouseAction) []Field {
	var fields []Field

	if action.StoryType != "" {
		fields = append(fields, Field{
			Name:   "Type",
			Value:  action.StoryType,
			Inline: true,
		})
	}

	if action.ProjectID > 0 {
		projectTypeID := fmt.Sprintf("%s:%d", "project", action.ProjectID)
		project := referencesByTypeID[projectTypeID]
		fields = append(fields, Field{
			Name:   "Project",
			Value:  project.Name,
			Inline: true,
		})
	}

	if action.MilestoneID > 0 {
		milestoneTypeID := fmt.Sprintf("%s:%d", "milestone", action.MilestoneID)
		milestone := referencesByTypeID[milestoneTypeID]
		fields = append(fields, Field{
			Name:   "Milestone",
			Value:  milestone.Name,
			Inline: true,
		})
	}

	if action.WorkflowStateID > 0 {
		workflowStateTypeID := fmt.Sprintf("%s:%d", "workflow-state", action.WorkflowStateID)
		workflowState := referencesByTypeID[workflowStateTypeID]
		fields = append(fields, Field{
			Name:   "State",
			Value:  workflowState.Name,
			Inline: true,
		})
	}

	if action.EpicID > 0 {
		epicTypeID := fmt.Sprintf("%s:%d", "epic", action.EpicID)
		epic := referencesByTypeID[epicTypeID]
		fields = append(fields, Field{
			Name:   "Epic",
			Value:  epic.Name,
			Inline: true,
		})
	}

	if action.IterationID > 0 {
		iterationTypeID := fmt.Sprintf("%s:%d", "iteration", action.IterationID)
		iteration := referencesByTypeID[iterationTypeID]
		fields = append(fields, Field{
			Name:   "Iteration",
			Value:  iteration.Name,
			Inline: true,
		})
	}

	if action.Estimate > 0 {
		fields = append(fields, Field{
			Name:   "Estimate",
			Value:  strconv.Itoa(action.Estimate),
			Inline: true,
		})
	}

	return fields
}

func getChangesFields(
	clubhouseApiClient *ClubhouseApiClient,
	referencesByTypeID map[string]ClubhouseReference,
	changes ClubhouseChanges,
) ([]Field, error) {
	var fields []Field

	if changes.Deadline != nil {
		oldDeadline := "No Date"
		if changes.Deadline.Old != nil {
			oldDeadline = changes.Deadline.Old.String()
		}
		newDeadline := "No Date"
		if changes.Deadline.New != nil {
			newDeadline = changes.Deadline.New.String()
		}
		fields = append(fields, Field{
			Name:  "Deadline",
			Value: fmt.Sprintf("%s -> %s", oldDeadline, newDeadline),
		})
	}

	if changes.EpicID != nil {
		oldEpicValue := "None"
		if changes.EpicID.Old != nil {
			oldEpicTypeID := fmt.Sprintf("%s:%d", "epic", *changes.EpicID.Old)
			oldEpic, ok := referencesByTypeID[oldEpicTypeID]
			if ok {
				oldEpicValue = oldEpic.Name
			} else {
				oldEpicValue = "Unknown"
			}
		}
		newEpicValue := "None"
		if changes.EpicID.New != nil {
			newEpicTypeID := fmt.Sprintf("%s:%d", "epic", *changes.EpicID.New)
			newEpic, ok := referencesByTypeID[newEpicTypeID]
			if ok {
				newEpicValue = newEpic.Name
			} else {
				newEpicValue = "Unknown"
			}
		}
		fields = append(fields, Field{
			Name:  "Epic",
			Value: fmt.Sprintf("%s -> %s", oldEpicValue, newEpicValue),
		})
	}

	if changes.Estimate != nil {
		oldEstimateValue := "Unestimated"
		if changes.Estimate.Old != nil {
			oldEstimateValue = strconv.Itoa(*changes.Estimate.Old)
		}
		newEstimateValue := "Unestimated"
		if changes.Estimate.New != nil {
			newEstimateValue = strconv.Itoa(*changes.Estimate.New)
		}
		fields = append(fields, Field{
			Name:  "Estimate",
			Value: fmt.Sprintf("%s -> %s", oldEstimateValue, newEstimateValue),
		})
	}

	if changes.IterationID != nil {
		oldIterationValue := "None"
		if changes.IterationID.Old != nil {
			oldIterationTypeID := fmt.Sprintf("%s:%d", "iteration", *changes.IterationID.Old)
			oldIteration, ok := referencesByTypeID[oldIterationTypeID]
			if ok {
				oldIterationValue = oldIteration.Name
			} else {
				oldIterationValue = "Unknown"
			}
		}
		newIterationValue := "None"
		if changes.IterationID.New != nil {
			newIterationTypeID := fmt.Sprintf("%s:%d", "iteration", *changes.IterationID.New)
			newIteration, ok := referencesByTypeID[newIterationTypeID]
			if ok {
				newIterationValue = newIteration.Name
			} else {
				newIterationValue = "Unknown"
			}
		}
		fields = append(fields, Field{
			Name:  "Iteration",
			Value: fmt.Sprintf("%s -> %s", oldIterationValue, newIterationValue),
		})
	}

	if changes.LabelIds != nil {
		if len(changes.LabelIds.Adds) > 0 {
			labelsAdded := make([]string, len(changes.LabelIds.Adds))
			for i, labelID := range changes.LabelIds.Adds {
				labelTypeID := fmt.Sprintf("%s:%d", "label", labelID)
				label, ok := referencesByTypeID[labelTypeID]
				if ok {
					labelsAdded[i] = label.Name
				}
			}

			if len(labelsAdded) > 0 {
				fields = append(fields, Field{
					Name:  "Label(s) Added",
					Value: strings.Join(labelsAdded, ", "),
				})
			}
		}

		if len(changes.LabelIds.Removes) > 0 {
			labelsRemoved := make([]string, len(changes.LabelIds.Removes))
			for i, labelID := range changes.LabelIds.Removes {
				labelTypeID := fmt.Sprintf("%s:%d", "label", labelID)
				label, ok := referencesByTypeID[labelTypeID]
				if ok {
					labelsRemoved[i] = label.Name
				}
			}

			if len(labelsRemoved) > 0 {
				fields = append(fields, Field{
					Name:  "Label(s) Removed",
					Value: strings.Join(labelsRemoved, ", "),
				})
			}
		}
	}

	if changes.OwnerIds != nil {
		if len(changes.OwnerIds.Adds) > 0 {
			ownersAdded := make([]string, len(changes.OwnerIds.Adds))
			for i, ownerID := range changes.OwnerIds.Adds {
				member, err := clubhouseApiClient.GetMember(ownerID)
				if err != nil {
					return []Field{}, err
				}
				ownersAdded[i] = member.Profile.Name
			}

			fields = append(fields, Field{
				Name:  "Owner(s) Added",
				Value: strings.Join(ownersAdded, ", "),
			})
		}

		if len(changes.OwnerIds.Removes) > 0 {
			ownersRemoved := make([]string, len(changes.OwnerIds.Removes))
			for i, ownerID := range changes.OwnerIds.Removes {
				member, err := clubhouseApiClient.GetMember(ownerID)
				if err != nil {
					return []Field{}, err
				}
				ownersRemoved[i] = member.Profile.Name
			}

			fields = append(fields, Field{
				Name:  "Owner(s) Removed",
				Value: strings.Join(ownersRemoved, ", "),
			})
		}
	}

	if changes.ProjectID != nil {
		oldProjectValue := "Unknown"
		oldProjectTypeID := fmt.Sprintf("%s:%d", "project", changes.ProjectID.Old)
		oldProject, ok := referencesByTypeID[oldProjectTypeID]
		if ok {
			oldProjectValue = oldProject.Name
		}

		newProjectValue := "Unknown"
		newProjectTypeID := fmt.Sprintf("%s:%d", "project", changes.ProjectID.New)
		newProject, ok := referencesByTypeID[newProjectTypeID]
		if ok {
			newProjectValue = newProject.Name
		}

		fields = append(fields, Field{
			Name:  "Project",
			Value: fmt.Sprintf("%s -> %s", oldProjectValue, newProjectValue),
		})
	}

	if changes.StoryType != nil {
		fields = append(fields, Field{
			Name:  "Type",
			Value: strings.Title(fmt.Sprintf("%s -> %s", changes.StoryType.Old, changes.StoryType.New)),
		})
	}

	if changes.Text != nil && changes.Text.Old != changes.Text.New {
		fields = append(fields, Field{
			Name: "Description",
			// Likely too long to include.
			Value: "(Edited)",
		})
	}

	if changes.WorkflowStateID != nil {
		oldWorkflowStateValue := "Unknown"
		oldWorkflowStateTypeID := fmt.Sprintf("%s:%d", "workflow-state", changes.WorkflowStateID.Old)
		oldWorkflowState, ok := referencesByTypeID[oldWorkflowStateTypeID]
		if ok {
			oldWorkflowStateValue = oldWorkflowState.Name
		}

		newWorkflowStateValue := "Unknown"
		newWorkflowStateTypeID := fmt.Sprintf("%s:%d", "workflow-state", changes.WorkflowStateID.New)
		newWorkflowState, ok := referencesByTypeID[newWorkflowStateTypeID]
		if ok {
			newWorkflowStateValue = newWorkflowState.Name
		}

		fields = append(fields, Field{
			Name:  "State",
			Value: strings.Title(fmt.Sprintf("%s -> %s", oldWorkflowStateValue, newWorkflowStateValue)),
		})
	}

	return fields, nil
}
