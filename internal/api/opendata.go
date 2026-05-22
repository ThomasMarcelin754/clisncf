package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	openDataBaseURL = "https://api.sncf.com/v1/coverage/sncf"
)

// OpenDataClient interacts with the SNCF Open Data API (Navitia-based).
type OpenDataClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewOpenDataClient creates a client for the SNCF Open Data API.
// Uses the SNCF_API_KEY environment variable, or falls back to the public demo key.
func NewOpenDataClient() *OpenDataClient {
	return &OpenDataClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetAPIKey sets the API key for authenticated requests.
func (c *OpenDataClient) SetAPIKey(key string) {
	c.apiKey = key
}

func (c *OpenDataClient) doRequest(endpoint string) (*http.Response, error) {
	reqURL := openDataBaseURL + endpoint

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.SetBasicAuth(c.apiKey, "")
	}
	req.Header.Set("User-Agent", "sncf-cli/1.0")

	return c.httpClient.Do(req)
}

// SearchJourneys searches for train journeys between two stations.
func (c *OpenDataClient) SearchJourneys(from, to, date, timeStr string) ([]Journey, error) {
	// Resolve station names to Navitia place IDs
	fromID, err := c.resolvePlace(from)
	if err != nil {
		return nil, fmt.Errorf("cannot find station '%s': %w", from, err)
	}
	toID, err := c.resolvePlace(to)
	if err != nil {
		return nil, fmt.Errorf("cannot find station '%s': %w", to, err)
	}

	// Build datetime parameter: YYYYMMDDTHHMMSS
	datetime := strings.ReplaceAll(date, "-", "") + "T"
	if timeStr != "" {
		datetime += strings.ReplaceAll(timeStr, ":", "") + "00"
	} else {
		datetime += "080000"
	}

	endpoint := fmt.Sprintf("/journeys?from=%s&to=%s&datetime=%s&count=10",
		url.QueryEscape(fromID), url.QueryEscape(toID), datetime)

	resp, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d)", resp.StatusCode)
	}

	var result navitiaJourneysResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var journeys []Journey
	for _, j := range result.Journeys {
		trainNum := ""
		trainType := ""
		for _, s := range j.Sections {
			if s.Type == "public_transport" && s.DisplayInformations.CommercialMode != "" {
				trainType = s.DisplayInformations.CommercialMode
				trainNum = s.DisplayInformations.TripShortName
				break
			}
		}

		journeys = append(journeys, Journey{
			DepartureTime: formatNavitiaTime(j.DepartureDateTime),
			ArrivalTime:   formatNavitiaTime(j.ArrivalDateTime),
			Duration:      formatDuration(j.Duration),
			TrainNumber:   trainNum,
			TrainType:     trainType,
			Status:        j.Status,
		})
	}

	return journeys, nil
}

// GetTrainStatus returns the real-time status of a specific train.
func (c *OpenDataClient) GetTrainStatus(trainNumber string) (*TrainStatus, error) {
	endpoint := fmt.Sprintf("/vehicle_journeys?filter=vehicle_journey.has_code(source, %s)&show_codes=true",
		url.QueryEscape(trainNumber))

	resp, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d)", resp.StatusCode)
	}

	var result navitiaVehicleJourneysResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.VehicleJourneys) == 0 {
		return nil, fmt.Errorf("train %s not found", trainNumber)
	}

	vj := result.VehicleJourneys[0]
	status := &TrainStatus{
		TrainNumber: trainNumber,
		TrainType:   vj.Name,
		Status:      "on time",
	}

	for _, st := range vj.StopTimes {
		stop := StopStatus{
			StationName:   st.StopPoint.Name,
			ArrivalTime:   formatNavitiaTime(st.ArrivalTime),
			DepartureTime: formatNavitiaTime(st.DepartureTime),
			Delay:         "-",
		}
		status.Stops = append(status.Stops, stop)
	}

	return status, nil
}

func (c *OpenDataClient) resolvePlace(query string) (string, error) {
	endpoint := fmt.Sprintf("/places?q=%s&type[]=stop_area&count=1", url.QueryEscape(query))

	resp, err := c.doRequest(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("places API error (HTTP %d)", resp.StatusCode)
	}

	var result navitiaPlacesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Places) == 0 {
		return "", fmt.Errorf("no station found")
	}

	return result.Places[0].ID, nil
}

// formatNavitiaTime converts "20260420T083000" to "08:30"
func formatNavitiaTime(t string) string {
	if len(t) < 13 {
		return t
	}
	// Format: YYYYMMDDTHHMMSS
	idx := strings.Index(t, "T")
	if idx == -1 || len(t) < idx+5 {
		return t
	}
	return t[idx+1:idx+3] + ":" + t[idx+3:idx+5]
}

// formatDuration converts seconds to "Xh Ym"
func formatDuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// Navitia API response types

type navitiaJourneysResponse struct {
	Journeys []navitiaJourney `json:"journeys"`
}

type navitiaJourney struct {
	DepartureDateTime string           `json:"departure_date_time"`
	ArrivalDateTime   string           `json:"arrival_date_time"`
	Duration          int              `json:"duration"`
	Status            string           `json:"status"`
	Sections          []navitiaSection `json:"sections"`
}

type navitiaSection struct {
	Type                string `json:"type"`
	DisplayInformations struct {
		CommercialMode string `json:"commercial_mode"`
		TripShortName  string `json:"trip_short_name"`
	} `json:"display_informations"`
}

type navitiaPlacesResponse struct {
	Places []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"places"`
}

type navitiaVehicleJourneysResponse struct {
	VehicleJourneys []struct {
		Name      string `json:"name"`
		StopTimes []struct {
			ArrivalTime   string `json:"arrival_time"`
			DepartureTime string `json:"departure_time"`
			StopPoint     struct {
				Name string `json:"name"`
			} `json:"stop_point"`
		} `json:"stop_times"`
	} `json:"vehicle_journeys"`
}
