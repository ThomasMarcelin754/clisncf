package api

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enetx/g"
	"github.com/enetx/surf"
)

func uuid4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

const (
	bffBaseURL = "https://www.sncf-connect.com/bff"
	bffKey     = "ah1MPO-izehIHD-QZZ9y88n-kku876"
)

// ConnectClient talks to the SNCF Connect BFF over a Chrome-fingerprinted
// TLS+HTTP/2 client (enetx/surf). Supports two auth modes:
//   - Token-based (headless login): oui-account-token + access-token headers
//   - Cookie-based (Chrome extract): Cookie header with full jar
type ConnectClient struct {
	cookie      string
	accessToken string
	idToken     string
	ddCookie    string
	http        *surf.Client
}

// NewConnectClient builds a client from a "name=value; ..." Cookie header (legacy).
func NewConnectClient(cookieHeader string) (*ConnectClient, error) {
	return &ConnectClient{cookie: cookieHeader, http: newSurfClient()}, nil
}

// NewTokenClient builds a client from headless login tokens (no cookies needed).
func NewTokenClient(accessToken, idToken string) (*ConnectClient, error) {
	return &ConnectClient{accessToken: accessToken, idToken: idToken, http: newSurfClient()}, nil
}

// Refresh calls /api/v2/authenticate/refresh with the body contract and
// returns updated tokens. The caller must persist these.
func (c *ConnectClient) Refresh(refreshToken string) (accessToken, idToken, newRefreshToken string, expireIn int, err error) {
	type refreshReq struct {
		RefreshToken string `json:"refreshToken"`
		TriggeredBy  string `json:"triggeredBy"`
	}
	resp, err := c.post("/api/v2/authenticate/refresh", refreshReq{
		RefreshToken: refreshToken,
		TriggeredBy:  "FOREGROUND",
	})
	if err != nil {
		return "", "", "", 0, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return "", "", "", 0, fmt.Errorf("refresh failed (%d): %.200s", int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		AccessToken  string `json:"accessToken"`
		IDToken      string `json:"idToken"`
		RefreshToken string `json:"refreshToken"`
		ExpireIn     int    `json:"expireIn"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return "", "", "", 0, err
	}
	c.accessToken = r.AccessToken
	c.idToken = r.IDToken
	return r.AccessToken, r.IDToken, r.RefreshToken, r.ExpireIn, nil
}

func (c *ConnectClient) headers(hasBody bool) map[string]string {
	h := map[string]string{
		"accept":           "application/json",
		"x-bff-key":        bffKey,
		"x-client-channel": "web",
		"x-client-app-id":  "front-web",
		"x-api-env":        "production",
		"x-market-locale":  "fr_FR",
		"x-app-version":    "v9.99.9",
		"origin":           "https://www.sncf-connect.com",
		"referer":          "https://www.sncf-connect.com/app/home",
	}
	if hasBody {
		h["content-type"] = "application/json"
	}
	if c.accessToken != "" {
		h["oui-account-token"] = c.idToken
		h["access-token"] = c.accessToken
	}
	if c.cookie != "" {
		h["cookie"] = c.cookie
	}
	return h
}

func (c *ConnectClient) post(path string, body any) (*surf.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	r := c.http.Post(g.String(bffBaseURL + path)).
		SetHeaders(c.headers(true)).
		Body(string(b)).
		Do()
	if r.IsErr() {
		return nil, r.Err()
	}
	return r.Unwrap(), nil
}

func (c *ConnectClient) get(path string) (*surf.Response, error) {
	r := c.http.Get(g.String(bffBaseURL + path)).
		SetHeaders(c.headers(false)).
		Do()
	if r.IsErr() {
		return nil, r.Err()
	}
	return r.Unwrap(), nil
}

func (c *ConnectClient) httpDelete(path string) (*surf.Response, error) {
	r := c.http.Delete(g.String(bffBaseURL + path)).
		SetHeaders(c.headers(false)).
		Do()
	if r.IsErr() {
		return nil, r.Err()
	}
	return r.Unwrap(), nil
}

func (c *ConnectClient) ensureDatadomeCookie() error {
	if c.ddCookie != "" {
		return nil
	}
	cookie, err := fetchDatadomeCookie(c.http)
	if err != nil {
		return fmt.Errorf("datadome: %w", err)
	}
	c.ddCookie = cookie
	return nil
}

func (c *ConnectClient) postWithDD(path string, body any) (*surf.Response, error) {
	if err := c.ensureDatadomeCookie(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	h := c.headers(true)
	if existing := h["cookie"]; existing != "" {
		h["cookie"] = existing + "; " + c.ddCookie
	} else {
		h["cookie"] = c.ddCookie
	}
	r := c.http.Post(g.String(bffBaseURL + path)).
		SetHeaders(h).
		Body(string(b)).
		Do()
	if r.IsErr() {
		return nil, r.Err()
	}
	return r.Unwrap(), nil
}

func (c *ConnectClient) GetCart() (*Cart, error) {
	resp, err := c.get("/api/v1/carts?withServices=false")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Items []struct {
			Trip struct {
				Title           string `json:"title"`
				PriceLabel      string `json:"priceLabel"`
				OutwardDateTime string `json:"outwardDateTimeLabel"`
				OriginDest      struct {
					OriginLabel      string `json:"originLabel"`
					DestinationLabel string `json:"destinationLabel"`
				} `json:"originDestination"`
			} `json:"trip"`
		} `json:"items"`
		TotalPriceLabel string `json:"totalPriceLabel"`
		TotalPrice      struct {
			Value    int    `json:"value"`
			Currency string `json:"currency"`
		} `json:"totalPrice"`
		NextStep string `json:"nextStep"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse cart: %w", err)
	}
	cart := &Cart{
		TotalLabel: r.TotalPriceLabel,
		TotalValue: r.TotalPrice.Value,
		Currency:   r.TotalPrice.Currency,
		NextStep:   r.NextStep,
	}
	for _, it := range r.Items {
		cart.Items = append(cart.Items, CartItem{
			Title:      it.Trip.Title,
			From:       it.Trip.OriginDest.OriginLabel,
			To:         it.Trip.OriginDest.DestinationLabel,
			When:       it.Trip.OutwardDateTime,
			PriceLabel: it.Trip.PriceLabel,
		})
	}
	return cart, nil
}

func (c *ConnectClient) GetProofs() ([]Proof, error) {
	resp, err := c.get("/api/v1/proofs-of-purchase")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	type lbl struct {
		Label string `json:"label"`
	}
	var r struct {
		TripsByMonth []struct {
			Title string `json:"title"`
			Trips []struct {
				Reference     string `json:"reference"`
				DepartureTime lbl    `json:"departureTimeLabel"`
				Origin        lbl    `json:"originLabel"`
				Destination   lbl    `json:"destinationLabel"`
				RouteLabel    string `json:"routeLabel"`
			} `json:"trips"`
		} `json:"tripsByMonth"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse proofs: %w", err)
	}
	var out []Proof
	for _, m := range r.TripsByMonth {
		for _, t := range m.Trips {
			out = append(out, Proof{
				Month:     m.Title,
				Date:      t.DepartureTime.Label,
				From:      t.Origin.Label,
				To:        t.Destination.Label,
				Route:     t.RouteLabel,
				Reference: t.Reference,
			})
		}
	}
	return out, nil
}

func classifyErr(status int, body []byte) error {
	if bytes.Contains(body, []byte("captcha-delivery")) {
		return fmt.Errorf("blocked by Datadome (HTTP %d) — run `sncfcli auth login` to re-authenticate", status)
	}
	if status == 401 {
		return fmt.Errorf("session expired (HTTP 401) — run `sncfcli auth login`")
	}
	return fmt.Errorf("BFF error (HTTP %d): %.180s", status, body)
}

type rawTrip struct {
	Trip struct {
		TripContent struct {
			OutwardJourneys []struct {
				Detail struct {
					JourneySummary struct {
						DepartureDate string `json:"departureDate"`
						OriginLabel   struct {
							Label string `json:"label"`
						} `json:"originLabel"`
						DestinationLabel struct {
							Label string `json:"label"`
						} `json:"destinationLabel"`
						TransportersList []struct {
							Number string `json:"number"`
						} `json:"transportersList"`
					} `json:"journeySummary"`
					References []string `json:"references"`
				} `json:"detail"`
			} `json:"outwardJourneys"`
		} `json:"tripContent"`
	} `json:"trip"`
}

func (rt rawTrip) toTrips() []Trip {
	var out []Trip
	for _, j := range rt.Trip.TripContent.OutwardJourneys {
		js := j.Detail.JourneySummary
		t := Trip{
			Date: js.DepartureDate,
			From: js.OriginLabel.Label,
			To:   js.DestinationLabel.Label,
		}
		if len(js.TransportersList) > 0 {
			t.TrainNumber = js.TransportersList[0].Number
		}
		if len(j.Detail.References) > 0 {
			t.BookingRef = j.Detail.References[0]
		}
		out = append(out, t)
	}
	return out
}

func (c *ConnectClient) GetTrips(past bool) ([]Trip, error) {
	filter := "UPCOMING"
	if past {
		filter = "PASSED"
	}
	type tripsReq struct {
		StatusFilter string `json:"statusFilter"`
	}
	resp, err := c.post("/api/v3/trips", tripsReq{StatusFilter: filter})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}

	var r struct {
		Trips            []rawTrip `json:"trips"`
		PassedTrips      []rawTrip `json:"passedTrips"`
		PreReservedTrips []rawTrip `json:"preReservedTrips"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse trips: %w", err)
	}

	src := append(r.Trips, r.PreReservedTrips...)
	if past {
		src = r.PassedTrips
	}
	var trips []Trip
	for _, rt := range src {
		trips = append(trips, rt.toTrips()...)
	}
	return trips, nil
}

// AutocompletePlace resolves a free-text term to a transport-place id+label.
func (c *ConnectClient) AutocompletePlace(term string) (id, label string, err error) {
	type acReq struct {
		SearchTerm         string `json:"searchTerm"`
		KeepStationsOnly   bool   `json:"keepStationsOnly"`
		ReturnsSuggestions bool   `json:"returnsSuggestions"`
	}
	resp, err := c.post("/api/v1/autocomplete", acReq{
		SearchTerm: term, KeepStationsOnly: true, ReturnsSuggestions: false,
	})
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return "", "", classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Places struct {
			TransportPlaces []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"transportPlaces"`
		} `json:"places"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return "", "", err
	}
	if len(r.Places.TransportPlaces) == 0 {
		return "", "", fmt.Errorf("no place found for %q", term)
	}
	p := r.Places.TransportPlaces[0]
	return p.ID, p.Label, nil
}

func (c *ConnectClient) SearchItineraries(fromTerm, toTerm string, when time.Time) ([]Proposal, error) {
	return c.searchItineraries(fromTerm, toTerm, when, nil)
}

// SearchWithCards searches itineraries with the account's discount cards injected.
func (c *ConnectClient) SearchWithCards(fromTerm, toTerm string, when time.Time) ([]Proposal, error) {
	var cards []map[string]any
	if acct, err := c.GetAccountDisplay(); err == nil && len(acct.DiscountCards) > 0 {
		for _, dc := range acct.DiscountCards {
			cards = append(cards, map[string]any{
				"code": dc.Code, "label": dc.Label, "number": dc.Number,
				"selected": true, "storedInAccount": true, "isExpiringSoon": false,
			})
		}
	}
	return c.searchItineraries(fromTerm, toTerm, when, cards)
}

// SearchWithCardsRaw returns the raw JSON from /api/v1/itineraries with discount cards.
func (c *ConnectClient) SearchWithCardsRaw(fromTerm, toTerm string, when time.Time) ([]byte, error) {
	var cards []map[string]any
	if acct, err := c.GetAccountDisplay(); err == nil && len(acct.DiscountCards) > 0 {
		for _, dc := range acct.DiscountCards {
			cards = append(cards, map[string]any{
				"code": dc.Code, "label": dc.Label, "number": dc.Number,
				"selected": true, "storedInAccount": true, "isExpiringSoon": false,
			})
		}
	}
	oID, oLabel, err := c.AutocompletePlace(fromTerm)
	if err != nil {
		return nil, fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	dID, dLabel, err := c.AutocompletePlace(toTerm)
	if err != nil {
		return nil, fmt.Errorf("destination %q: %w", toTerm, err)
	}
	place := func(id, lbl string) map[string]any {
		return map[string]any{"label": lbl, "id": id, "codes": []any{}, "geolocation": false}
	}
	dc := []any{}
	for _, c := range cards {
		dc = append(dc, c)
	}
	req := map[string]any{
		"schedule": map[string]any{"outward": map[string]any{
			"date": when.UTC().Format("2006-01-02T15:04:05.000Z"), "arrivalAt": false}},
		"mainJourney": map[string]any{"origin": place(oID, oLabel), "destination": place(dID, dLabel)},
		"passengers": []any{map[string]any{
			"id": uuid4(), "typology": "ADULT", "age": 30,
			"withoutSeatAssignment": false, "hasDisability": false,
			"hasWheelchair": false, "discountCards": dc}},
		"pets": []any{}, "itineraryId": uuid4(), "forceDisplayResults": false,
		"trainExpected": true, "wishBike": false, "strictMode": false,
		"directJourney": false, "transporterLabels": []any{}, "metadataY": map[string]any{},
		"userNavigation": []any{"IS_NOT_BUSINESS"}, "branch": "SHOP",
	}
	resp, err := c.postWithDD("/api/v1/itineraries", req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

func (c *ConnectClient) searchItineraries(fromTerm, toTerm string, when time.Time, discountCards []map[string]any) ([]Proposal, error) {
	oID, oLabel, err := c.AutocompletePlace(fromTerm)
	if err != nil {
		return nil, fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	dID, dLabel, err := c.AutocompletePlace(toTerm)
	if err != nil {
		return nil, fmt.Errorf("destination %q: %w", toTerm, err)
	}
	place := func(id, lbl string) map[string]any {
		return map[string]any{"label": lbl, "id": id, "codes": []any{}, "geolocation": false}
	}

	dc := []any{}
	for _, c := range discountCards {
		dc = append(dc, c)
	}

	itineraryID := uuid4()
	req := map[string]any{
		"schedule": map[string]any{"outward": map[string]any{
			"date": when.UTC().Format("2006-01-02T15:04:05.000Z"), "arrivalAt": false}},
		"mainJourney": map[string]any{"origin": place(oID, oLabel), "destination": place(dID, dLabel)},
		"passengers": []any{map[string]any{
			"id": uuid4(), "typology": "ADULT", "age": 30,
			"withoutSeatAssignment": false, "hasDisability": false,
			"hasWheelchair": false, "discountCards": dc}},
		"pets": []any{}, "itineraryId": itineraryID, "forceDisplayResults": false,
		"trainExpected": true, "wishBike": false, "strictMode": false,
		"directJourney": false, "transporterLabels": []any{}, "metadataY": map[string]any{},
		"userNavigation": []any{"IS_NOT_BUSINESS"}, "branch": "SHOP",
	}

	resp, err := c.postWithDD("/api/v1/itineraries", req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}

	type rawSegmentInfo struct {
		SegmentID string `json:"segmentId"`
	}
	type rawPlacementSegment struct {
		SegmentInfo rawSegmentInfo `json:"segmentInfo"`
	}
	type rawSegmentPlacements struct {
		Segments []rawPlacementSegment `json:"segments"`
	}
	type rawOffer struct {
		ID     string `json:"id"`
		Price  string `json:"priceLabel"`
		Header struct {
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"header"`
		SegmentPlacements *rawSegmentPlacements `json:"segmentPlacements"`
	}
	type rawComfort struct {
		Offers []rawOffer `json:"offers"`
	}
	type rawProposal struct {
		ID        string `json:"id"`
		TravelID  string `json:"travelId"`
		Departure struct {
			Station string `json:"originStationLabel"`
			Time    string `json:"timeLabel"`
		} `json:"departure"`
		Arrival struct {
			Station string `json:"destinationStationLabel"`
			Time    string `json:"timeLabel"`
		} `json:"arrival"`
		Duration    string `json:"durationLabel"`
		Transporter string `json:"transporterDescription"`
		BestPrice   string `json:"bestPriceLabel"`
		Status      struct {
			IsBookable bool `json:"isBookable"`
		} `json:"status"`
		Second *rawComfort `json:"secondComfortClassOffers"`
		First  *rawComfort `json:"firstComfortClassOffers"`
	}
	type proposalsWrap struct {
		Proposals []rawProposal `json:"proposals"`
	}
	var r struct {
		LongDistance *struct {
			Proposals *proposalsWrap `json:"proposals"`
		} `json:"longDistance"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse itineraries: %w", err)
	}
	if r.LongDistance == nil || r.LongDistance.Proposals == nil {
		return nil, nil
	}

	var out []Proposal
	for _, p := range r.LongDistance.Proposals.Proposals {
		prop := Proposal{
			ID: p.ID, TravelID: p.TravelID, ItineraryID: itineraryID,
			Departure: p.Departure.Time, Arrival: p.Arrival.Time,
			Duration: p.Duration, Origin: p.Departure.Station,
			Destination: p.Arrival.Station, Transporter: p.Transporter,
			BestPrice: p.BestPrice, IsBookable: p.Status.IsBookable,
		}
		appendOffers := func(src *rawComfort, class string) {
			if src == nil {
				return
			}
			for _, o := range src.Offers {
				of := Offer{
					ID: o.ID, Price: o.Price, FareName: o.Header.Title, Class: class,
				}
				if o.SegmentPlacements != nil && len(o.SegmentPlacements.Segments) > 0 {
					of.SegmentID = o.SegmentPlacements.Segments[0].SegmentInfo.SegmentID
				}
				prop.Offers = append(prop.Offers, of)
			}
		}
		appendOffers(p.Second, "2nd")
		appendOffers(p.First, "1st")
		out = append(out, prop)
	}
	return out, nil
}

// GetAccountDisplay returns the full account profile (discountCards, personal info, etc.).
func (c *ConnectClient) GetAccountDisplay() (*AccountDisplay, error) {
	resp, err := c.get("/api/v1/accounts/display")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Display struct {
			Account AccountDisplay `json:"account"`
		} `json:"display"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse account display: %w", err)
	}
	return &r.Display.Account, nil
}

// GetCardCode returns the loyalty card QR code (base64 PNG).
func (c *ConnectClient) GetCardCode() (*CardCode, error) {
	resp, err := c.get("/api/v1/accounts/card-code")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Display struct {
			HasCode struct {
				CardCode string `json:"cardCode"`
			} `json:"hasCode"`
		} `json:"display"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse card-code: %w", err)
	}
	return &CardCode{QRCode: r.Display.HasCode.CardCode}, nil
}

// GetCompanions returns the list of travel companions (from /accounts/display).
func (c *ConnectClient) GetCompanions() ([]Companion, error) {
	acct, err := c.GetAccountDisplay()
	if err != nil {
		return nil, err
	}
	return acct.Companions, nil
}


// GetLowPriceAlerts returns price-watch alerts.
func (c *ConnectClient) GetLowPriceAlerts() ([]LowPriceAlert, error) {
	resp, err := c.get("/api/v1/alerting/low-price-alert")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Alerts []LowPriceAlert `json:"alerts"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return r.Alerts, nil
}

// GetSalesOpeningAlerts returns sales-opening alerts.
func (c *ConnectClient) GetSalesOpeningAlerts() ([]SalesOpeningAlert, error) {
	resp, err := c.get("/api/v1/alerting/sales-openings")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Alerts []SalesOpeningAlert `json:"alerts"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return r.Alerts, nil
}

// GetFullTrainAlerts returns full-train alerts.
func (c *ConnectClient) GetFullTrainAlerts() ([]FullTrainAlert, error) {
	resp, err := c.get("/api/v1/alerting/full-train")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Alerts []FullTrainAlert `json:"alerts"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return r.Alerts, nil
}

// GetFavoritePlaces returns saved favorite places.
func (c *ConnectClient) GetFavoritePlaces() ([]FavoritePlace, error) {
	resp, err := c.get("/api/v1/favorite-places")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Places []FavoritePlace `json:"places"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return r.Places, nil
}

// GetTrafficInfo returns current traffic disruptions (public, ~900KB).
func (c *ConnectClient) GetTrafficInfo() ([]byte, error) {
	resp, err := c.get("/api/v1/trafficinfo")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetTripByID returns details for a single trip.
func (c *ConnectClient) GetTripByID(tripID string) ([]Trip, error) {
	resp, err := c.get("/api/v3/trip/" + tripID)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Trips       []rawTrip `json:"trips"`
		PassedTrips []rawTrip `json:"passedTrips"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	var trips []Trip
	for _, rt := range append(r.Trips, r.PassedTrips...) {
		trips = append(trips, rt.toTrips()...)
	}
	return trips, nil
}

// FindTripByInventory finds a trip by reference + train number + departure date.
func (c *ConnectClient) FindTripByInventory(ref, trainNum, date string) ([]Trip, error) {
	type findReq struct {
		Reference     string `json:"reference"`
		TrainNumber   string `json:"trainNumber"`
		DepartureDate string `json:"departureDate"`
	}
	resp, err := c.post("/api/v3/trips/trips-from-inventories", findReq{
		Reference: ref, TrainNumber: trainNum, DepartureDate: date,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Trips       []rawTrip `json:"trips"`
		PassedTrips []rawTrip `json:"passedTrips"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	var trips []Trip
	for _, rt := range append(r.Trips, r.PassedTrips...) {
		trips = append(trips, rt.toTrips()...)
	}
	return trips, nil
}

// GetCatalog returns the product catalog (discount cards, TER, etc.).
func (c *ConnectClient) GetCatalog() ([]CatalogNode, error) {
	resp, err := c.get("/api/v1/catalog")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Nodes map[string]CatalogNode `json:"nodes"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	var out []CatalogNode
	for _, n := range r.Nodes {
		out = append(out, n)
	}
	return out, nil
}

// GetCartRaw returns the raw JSON cart response (for extracting groupId, tripId, etc.).
func (c *ConnectClient) GetCartRaw() ([]byte, error) {
	resp, err := c.get("/api/v1/carts?withServices=false")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GenerateProof sends a proof-of-purchase justificatif by email for the given trip IDs.
func (c *ConnectClient) GenerateProof(email, name string, tripIDs []string) error {
	type generateReq struct {
		Email    string   `json:"email"`
		Name     string   `json:"name"`
		TripsIDs []string `json:"tripsIds"`
	}
	resp, err := c.post("/api/v3/proofs-of-purchase/generate", generateReq{
		Email: email, Name: name, TripsIDs: tripIDs,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// SetTripMotive sets a trip's motive (business vs personal).
func (c *ConnectClient) SetTripMotive(tripID string, isBusiness bool) error {
	type motiveReq struct {
		IsBusiness bool `json:"isBusiness"`
	}
	resp, err := c.post("/api/v3/trip/"+tripID+"/motives", motiveReq{IsBusiness: isBusiness})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// GetTripsFilter returns available trip filter options (ALL/BUSINESS/PERSONAL).
func (c *ConnectClient) GetTripsFilter() (*TripsFilter, error) {
	resp, err := c.get("/api/v3/trips/filter")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r TripsFilter
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// GetOptIns returns notification preferences.
func (c *ConnectClient) GetOptIns() (*OptInsPage, error) {
	resp, err := c.get("/api/v1/accounts/optIns")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Page OptInsPage `json:"page"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return &r.Page, nil
}

// GetOffersSettings returns offer/alert settings per city.
func (c *ConnectClient) GetOffersSettings() ([]OfferCategory, error) {
	resp, err := c.get("/api/v1/accounts/offers/settings")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Categories []OfferCategory `json:"categories"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return r.Categories, nil
}

// GetProfilePicture returns the profile picture as base64 JPEG.
func (c *ConnectClient) GetProfilePicture() (string, error) {
	resp, err := c.get("/api/v1/accounts/picture")
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return "", classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Picture string `json:"picture"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return "", err
	}
	return r.Picture, nil
}

// GetBoards returns real-time departure/arrival boards for a station.
// stationID must be in RESARAIL format (e.g. "RESARAIL_STA_8768600") from autocomplete.
func (c *ConnectClient) GetBoards(stationID string) (*BoardsResult, error) {
	resp, err := c.get("/api/v3/boards/" + stationID)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r BoardsResult
	if err := resp.Body.JSON(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdatePersonalInfo updates the account holder's personal info.
func (c *ConnectClient) UpdatePersonalInfo(firstName, lastName, birthDate, civility, phoneNumber string) error {
	type updateReq struct {
		FirstName   string `json:"firstName"`
		LastName    string `json:"lastName"`
		BirthDate   string `json:"birthDate"`
		Civility    string `json:"civility"`
		PhoneNumber string `json:"phoneNumber"`
	}
	resp, err := c.post("/api/v1/accounts/personal-info", updateReq{
		FirstName: firstName, LastName: lastName, BirthDate: birthDate,
		Civility: civility, PhoneNumber: phoneNumber,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// UpdateAddress updates the account holder's address.
func (c *ConnectClient) UpdateAddress(mainAddress, complement, city, zipCode, countryID string) error {
	type updateReq struct {
		MainAddress string `json:"mainAddress"`
		Complement  string `json:"complement"`
		City        string `json:"city"`
		ZipCode     string `json:"zipCode"`
		CountryID   string `json:"countryId"`
	}
	resp, err := c.post("/api/v1/accounts/address", updateReq{
		MainAddress: mainAddress, Complement: complement, City: city,
		ZipCode: zipCode, CountryID: countryID,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// UpdateProofsSettings updates justificatif email settings.
func (c *ConnectClient) UpdateProofsSettings(emailPro string, autoSend bool) error {
	type updateReq struct {
		EmailPro           string `json:"emailPro"`
		SendAutomaticEmail bool   `json:"sendAutomaticEmail"`
	}
	resp, err := c.post("/api/v1/accounts/proofs-settings", updateReq{
		EmailPro: emailPro, SendAutomaticEmail: autoSend,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// CreateLowPriceAlert creates a low-price alert. Origin/destination resolved from autocomplete.
func (c *ConnectClient) CreateLowPriceAlert(fromTerm, toTerm, startDate, endDate, email string, maxPrice int) error {
	oID, oLabel, oCodes, err := c.autocompletePlaceFull(fromTerm)
	if err != nil {
		return fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	dID, dLabel, dCodes, err := c.autocompletePlaceFull(toTerm)
	if err != nil {
		return fmt.Errorf("destination %q: %w", toTerm, err)
	}

	type place struct {
		Label       string `json:"label"`
		ID          string `json:"id"`
		Codes       []code `json:"codes"`
		Geolocation bool   `json:"geolocation"`
	}
	type timeSlotEnd struct {
		Hour  int    `json:"hour"`
		Label string `json:"label"`
	}
	type timeSlot struct {
		Start timeSlotEnd `json:"start"`
		End   timeSlotEnd `json:"end"`
	}
	type schedule struct {
		TimeSlot  timeSlot `json:"timeSlot"`
		DayOfWeek string   `json:"dayOfWeek,omitempty"`
	}
	type dateRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	type alertReq struct {
		Origin             place     `json:"origin"`
		Destination        place     `json:"destination"`
		MaxPrice           int       `json:"maxPrice"`
		DateRange          dateRange `json:"dateRange"`
		Email              string    `json:"email"`
		Outward            schedule  `json:"outward"`
		Inward             schedule  `json:"inward"`
		ShouldNotifyMobile bool      `json:"shouldNotifyMobile"`
	}

	body := alertReq{
		Origin:      place{Label: oLabel, ID: oID, Codes: oCodes, Geolocation: false},
		Destination: place{Label: dLabel, ID: dID, Codes: dCodes, Geolocation: false},
		MaxPrice:    maxPrice,
		DateRange:   dateRange{Start: startDate, End: endDate},
		Email:       email,
		Outward: schedule{TimeSlot: timeSlot{
			Start: timeSlotEnd{Hour: 4, Label: "04:00"},
			End:   timeSlotEnd{Hour: 23, Label: "23:00"},
		}},
		Inward: schedule{TimeSlot: timeSlot{
			Start: timeSlotEnd{Hour: 4, Label: "04:00"},
			End:   timeSlotEnd{Hour: 23, Label: "23:00"},
		}},
		ShouldNotifyMobile: false,
	}

	resp, err := c.post("/api/v1/alerting/low-price-alert", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// CheckLowPriceOD checks OD eligibility for a low-price alert.
func (c *ConnectClient) CheckLowPriceOD(fromTerm, toTerm string) (*LowPriceODEligibility, error) {
	oID, oLabel, oCodes, err := c.autocompletePlaceFull(fromTerm)
	if err != nil {
		return nil, fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	dID, dLabel, dCodes, err := c.autocompletePlaceFull(toTerm)
	if err != nil {
		return nil, fmt.Errorf("destination %q: %w", toTerm, err)
	}

	type place struct {
		Label       string `json:"label"`
		ID          string `json:"id"`
		Codes       []code `json:"codes"`
		Geolocation bool   `json:"geolocation"`
	}
	type odReq struct {
		Origin      place `json:"origin"`
		Destination place `json:"destination"`
		RoundTrip   bool  `json:"roundTrip"`
	}
	body := odReq{
		Origin:      place{Label: oLabel, ID: oID, Codes: oCodes, Geolocation: false},
		Destination: place{Label: dLabel, ID: dID, Codes: dCodes, Geolocation: false},
		RoundTrip:   false,
	}

	resp, err := c.post("/api/v1/alerting/low-price-alert/check/od", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r LowPriceODEligibility
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse check/od response: %w", err)
	}
	return &r, nil
}

// CheckLowPriceSchedule checks schedule eligibility and returns the price range (min/max/avg).
func (c *ConnectClient) CheckLowPriceSchedule(fromTerm, toTerm, startDate, endDate string) (*LowPriceScheduleEligibility, error) {
	oID, oLabel, oCodes, err := c.autocompletePlaceFull(fromTerm)
	if err != nil {
		return nil, fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	dID, dLabel, dCodes, err := c.autocompletePlaceFull(toTerm)
	if err != nil {
		return nil, fmt.Errorf("destination %q: %w", toTerm, err)
	}

	type place struct {
		Label       string `json:"label"`
		ID          string `json:"id"`
		Codes       []code `json:"codes"`
		Geolocation bool   `json:"geolocation"`
	}
	type timeSlotEnd struct {
		Hour  int    `json:"hour"`
		Label string `json:"label"`
	}
	type timeSlot struct {
		Start timeSlotEnd `json:"start"`
		End   timeSlotEnd `json:"end"`
	}
	type schedule struct {
		TimeSlot  timeSlot `json:"timeSlot"`
		DayOfWeek string   `json:"dayOfWeek,omitempty"`
	}
	type dateRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	type schedReq struct {
		Origin      place     `json:"origin"`
		Destination place     `json:"destination"`
		Outward     schedule  `json:"outward"`
		Inward      schedule  `json:"inward"`
		DateRange   dateRange `json:"dateRange"`
	}

	body := schedReq{
		Origin:      place{Label: oLabel, ID: oID, Codes: oCodes, Geolocation: false},
		Destination: place{Label: dLabel, ID: dID, Codes: dCodes, Geolocation: false},
		Outward: schedule{TimeSlot: timeSlot{
			Start: timeSlotEnd{Hour: 4, Label: "04:00"},
			End:   timeSlotEnd{Hour: 23, Label: "23:00"},
		}},
		Inward: schedule{TimeSlot: timeSlot{
			Start: timeSlotEnd{Hour: 4, Label: "04:00"},
			End:   timeSlotEnd{Hour: 23, Label: "23:00"},
		}},
		DateRange: dateRange{Start: startDate, End: endDate},
	}

	resp, err := c.post("/api/v1/alerting/low-price-alert/check/schedule", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r LowPriceScheduleEligibility
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse check/schedule response: %w", err)
	}
	return &r, nil
}

// GetCalendarBestPrices returns the best price per day for a given OD and date range.
func (c *ConnectClient) GetCalendarBestPrices(fromTerm, toTerm string, firstDate, lastDate string) ([]BestPriceDay, error) {
	oID, oLabel, err := c.AutocompletePlace(fromTerm)
	if err != nil {
		return nil, fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	dID, dLabel, err := c.AutocompletePlace(toTerm)
	if err != nil {
		return nil, fmt.Errorf("destination %q: %w", toTerm, err)
	}

	place := func(id, lbl string) map[string]any {
		return map[string]any{"label": lbl, "id": id, "codes": []any{}, "geolocation": false}
	}
	body := map[string]any{
		"itineraryRequest": map[string]any{
			"schedule": map[string]any{"outward": map[string]any{
				"date": firstDate + "T07:00:00.000Z", "arrivalAt": false}},
			"mainJourney":        map[string]any{"origin": place(oID, oLabel), "destination": place(dID, dLabel)},
			"passengers":         []any{map[string]any{"id": uuid4(), "typology": "ADULT", "age": 30, "withoutSeatAssignment": false, "hasDisability": false, "hasWheelchair": false, "discountCards": []any{}}},
			"pets":               []any{},
			"itineraryId":        uuid4(),
			"forceDisplayResults": false,
			"trainExpected":      true,
			"wishBike":           false,
			"strictMode":         false,
			"directJourney":      false,
			"transporterLabels":  []any{},
			"metadataY":          map[string]any{},
			"userNavigation":     []any{"IS_NOT_BUSINESS"},
			"branch":            "SHOP",
		},
		"firstDate":          firstDate + "T00:00:00.000Z",
		"lastDate":           lastDate + "T00:00:00.000Z",
		"bestPriceThreshold": 35,
		"promoState":         "DEFAULT",
	}

	resp, err := c.post("/api/v1/edito/calendar", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		raw := string(resp.Body.Bytes().Ok())
		if bytes.Contains([]byte(raw), []byte("not currently covered")) {
			return nil, fmt.Errorf("this route is not covered by the best-price calendar")
		}
		return nil, classifyErr(int(resp.StatusCode), []byte(raw))
	}
	var r struct {
		FrontCalendarBestPrices []BestPriceDay `json:"frontCalendarBestPrices"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse calendar best prices: %w", err)
	}
	return r.FrontCalendarBestPrices, nil
}

// CreateSalesOpeningAlert creates a sales-opening (booking) alert.
func (c *ConnectClient) CreateSalesOpeningAlert(fromTerm, toTerm, date, email string) error {
	_, _, oCodes, err := c.autocompletePlaceFull(fromTerm)
	if err != nil {
		return fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	_, _, dCodes, err := c.autocompletePlaceFull(toTerm)
	if err != nil {
		return fmt.Errorf("destination %q: %w", toTerm, err)
	}

	oRR := resarailCode(oCodes)
	dRR := resarailCode(dCodes)
	if oRR == "" || dRR == "" {
		return fmt.Errorf("could not find RESARAIL codes for %q / %q", fromTerm, toTerm)
	}

	type alertInner struct {
		OriginDestinationKey string   `json:"originDestinationKey"`
		Date                 string   `json:"date"`
		Transporters         []string `json:"transporters"`
		OptIn                string   `json:"optIn"`
	}
	type alertReq struct {
		Alert              alertInner `json:"alert"`
		Email              string     `json:"email"`
		ShouldNotifyMobile bool       `json:"shouldNotifyMobile"`
	}

	body := alertReq{
		Alert: alertInner{
			OriginDestinationKey: oRR + "-" + dRR,
			Date:                 date,
			Transporters:         []string{"TGV"},
			OptIn:                "",
		},
		Email:              email,
		ShouldNotifyMobile: false,
	}

	resp, err := c.post("/api/v1/alerting/sales-opening", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

func resarailCode(codes []code) string {
	for _, c := range codes {
		if c.Type == "RESARAIL" {
			return c.Value
		}
	}
	return ""
}

// autocompletePlaceFull resolves a term to id, label, and codes array.
func (c *ConnectClient) autocompletePlaceFull(term string) (id, label string, codes []code, err error) {
	type acReq struct {
		SearchTerm         string `json:"searchTerm"`
		KeepStationsOnly   bool   `json:"keepStationsOnly"`
		ReturnsSuggestions bool   `json:"returnsSuggestions"`
	}
	resp, err := c.post("/api/v1/autocomplete", acReq{
		SearchTerm: term, KeepStationsOnly: false, ReturnsSuggestions: false,
	})
	if err != nil {
		return "", "", nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return "", "", nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var r struct {
		Places struct {
			TransportPlaces []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
				Codes []code `json:"codes"`
			} `json:"transportPlaces"`
		} `json:"places"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return "", "", nil, err
	}
	if len(r.Places.TransportPlaces) == 0 {
		return "", "", nil, fmt.Errorf("no place found for %q", term)
	}
	p := r.Places.TransportPlaces[0]
	return p.ID, p.Label, p.Codes, nil
}

type code struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// AddCompanion adds a travel companion to the account.
func (c *ConnectClient) AddCompanion(firstName, lastName, dateOfBirth, email, civility string) error {
	type companion struct {
		FirstName   string `json:"firstName"`
		LastName    string `json:"lastName"`
		DateOfBirth string `json:"dateOfBirth"`
		Email       string `json:"email"`
		Civility    string `json:"civility"`
		KiOuiType   string `json:"kiOuiType"`
	}
	type addCompanionReq struct {
		Companion    companion `json:"companion"`
		DiscountCards []any    `json:"discountCards"`
		LoyaltyCards  []any    `json:"loyaltyCards"`
	}
	resp, err := c.post("/api/v1/accounts/companions/add", addCompanionReq{
		Companion: companion{
			FirstName:   firstName,
			LastName:    lastName,
			DateOfBirth: dateOfBirth,
			Email:       email,
			Civility:    civility,
			KiOuiType:   "NON_BENEFICIARY",
		},
		DiscountCards: []any{},
		LoyaltyCards:  []any{},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// DeleteCompanion removes a travel companion by ID.
func (c *ConnectClient) DeleteCompanion(companionID string) error {
	type deleteCompanionReq struct {
		CompanionID string `json:"companionId"`
	}
	resp, err := c.post("/api/v1/accounts/companions/delete", deleteCompanionReq{
		CompanionID: companionID,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// AddPet adds a pet to the account.
func (c *ConnectClient) AddPet(name, petType string) error {
	type pet struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type addPetReq struct {
		Pet pet `json:"pet"`
	}
	resp, err := c.post("/api/v1/accounts/pets/add", addPetReq{
		Pet: pet{Name: name, Type: petType},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// DeletePet removes a pet by ID.
func (c *ConnectClient) DeletePet(petID string) error {
	type deletePetReq struct {
		PetID string `json:"petId"`
	}
	resp, err := c.post("/api/v1/accounts/pets/delete", deletePetReq{
		PetID: petID,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// DeleteAlert removes an alert by type and ID.
// alertType must be "low-price", "sales-opening", or "full-train".
func (c *ConnectClient) DeleteAlert(alertType, alertID string) error {
	type emptyReq struct{}
	resp, err := c.post("/api/v1/alerting/"+alertType+"/"+alertID, emptyReq{})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// UpdateEnterpriseCode sets the enterprise code on the account.
func (c *ConnectClient) UpdateEnterpriseCode(code string, enable bool) error {
	type req struct {
		EnableEnterpriseFeatures bool   `json:"enableEnterpriseFeatures"`
		EnterpriseCode           string `json:"enterpriseCode"`
	}
	resp, err := c.post("/api/v1/accounts/enterprise-code", req{
		EnableEnterpriseFeatures: enable, EnterpriseCode: code,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// UpdateInfoPro updates the professional info on the account.
func (c *ConnectClient) UpdateInfoPro(address, companyName, emailPro, siren, tva string) error {
	type req struct {
		Address     string `json:"address"`
		CompanyName string `json:"companyName"`
		EmailPro    string `json:"emailPro"`
		SirenNumber string `json:"sirenNumber"`
		TvaNumber   string `json:"tvaNumber"`
	}
	resp, err := c.post("/api/v1/accounts/info-pro", req{
		Address: address, CompanyName: companyName, EmailPro: emailPro,
		SirenNumber: siren, TvaNumber: tva,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// UpdateNewsletter updates the generic newsletter subscription.
func (c *ConnectClient) UpdateNewsletter(subscribe bool, market string) error {
	type req struct {
		IsSubscriber bool   `json:"isSubscriber"`
		Market       string `json:"market"`
	}
	resp, err := c.post("/api/v1/accounts/newsletter-generic", req{
		IsSubscriber: subscribe, Market: market,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// UpdateOptIn toggles a notification preference.
func (c *ConnectClient) UpdateOptIn(optInID string, enable bool) error {
	type req struct {
		NewState bool   `json:"newState"`
		OptIn    string `json:"optIn"`
	}
	resp, err := c.post("/api/v1/accounts/optIn", req{
		NewState: enable, OptIn: optInID,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// UpdatePaymentCard updates a saved payment card's name and main status.
func (c *ConnectClient) UpdatePaymentCard(cardID, cardName string, mainCard bool) error {
	type req struct {
		CardID   string `json:"cardId"`
		CardName string `json:"cardName"`
		MainCard bool   `json:"mainCard"`
	}
	resp, err := c.post("/api/v1/accounts/payment-cards", req{
		CardID: cardID, CardName: cardName, MainCard: mainCard,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// SendTripsHistory sends trip history to the given email for a date.
func (c *ConnectClient) SendTripsHistory(date, email string) error {
	type req struct {
		Date  string `json:"date"`
		Email string `json:"email"`
	}
	resp, err := c.post("/api/v3/proofs-of-purchase/history", req{
		Date: date, Email: email,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// SearchItinerariesRaw returns the raw JSON from /api/v1/itineraries for the booking funnel.
// Needed to extract itineraryId + selectedTravelId for Book().
func (c *ConnectClient) SearchItinerariesRaw(fromTerm, toTerm string, when time.Time) ([]byte, error) {
	oID, oLabel, err := c.AutocompletePlace(fromTerm)
	if err != nil {
		return nil, fmt.Errorf("origin %q: %w", fromTerm, err)
	}
	dID, dLabel, err := c.AutocompletePlace(toTerm)
	if err != nil {
		return nil, fmt.Errorf("destination %q: %w", toTerm, err)
	}
	place := func(id, lbl string) map[string]any {
		return map[string]any{"label": lbl, "id": id, "codes": []any{}, "geolocation": false}
	}
	req := map[string]any{
		"schedule":    map[string]any{"outward": map[string]any{"date": when.UTC().Format("2006-01-02T15:04:05.000Z"), "arrivalAt": false}},
		"mainJourney": map[string]any{"origin": place(oID, oLabel), "destination": place(dID, dLabel)},
		"passengers": []any{map[string]any{
			"id": uuid4(), "typology": "ADULT", "age": 30,
			"withoutSeatAssignment": false, "hasDisability": false,
			"hasWheelchair": false, "discountCards": []any{}}},
		"pets": []any{}, "itineraryId": uuid4(), "forceDisplayResults": false,
		"trainExpected": true, "wishBike": false, "strictMode": false,
		"directJourney": false, "transporterLabels": []any{}, "metadataY": map[string]any{},
		"userNavigation": []any{"IS_NOT_BUSINESS"}, "branch": "SHOP",
	}
	resp, err := c.postWithDD("/api/v1/itineraries", req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// PlacementPrefs holds seat preference codes for a segment.
type PlacementPrefs struct {
	SegmentID  string
	SeatCode   string // e.g. FENETRE, COULOIR, SOLO, CLUB_QUATRE, DUO_COTE_A_COTE
	DeckCode   string // e.g. BAS, HAUT, ANY
}

// Book adds a selected travel to the server-side cart.
// itineraryId = client-generated UUID from search, selectedTravelId = from itineraries response.
func (c *ConnectClient) Book(itineraryID, selectedTravelID string) error {
	return c.BookWithPlacement(itineraryID, selectedTravelID, nil)
}

// BookWithPlacement adds a travel to cart with optional seat preferences.
func (c *ConnectClient) BookWithPlacement(itineraryID, selectedTravelID string, prefs *PlacementPrefs) error {
	type bookReq struct {
		ItineraryID                      string `json:"itineraryId"`
		SelectedTravelID                 string `json:"selectedTravelId"`
		DiscountCardPushSelected         bool   `json:"discountCardPushSelected"`
		SelectedPlacements               any    `json:"selectedPlacements"`
		SegmentSelectedAdditionalServices []any  `json:"segmentSelectedAdditionalServices"`
	}

	placements := map[string][]any{
		"inwardSelectedPlacement":  {},
		"outwardSelectedPlacement": {},
	}
	if prefs != nil && prefs.SegmentID != "" {
		seg := map[string]any{
			"segmentId": prefs.SegmentID,
			"selectedPreferencesPlacementMode": map[string]any{
				"placementChoices":  []string{prefs.SeatCode},
				"facingForward":     nil,
				"comfortSpaceId":    nil,
				"berthLevelChoices": []any{},
			},
		}
		placements["outwardSelectedPlacement"] = []any{seg}
	}

	body := bookReq{
		ItineraryID:                      itineraryID,
		SelectedTravelID:                 selectedTravelID,
		SelectedPlacements:               placements,
		SegmentSelectedAdditionalServices: []any{},
	}
	resp, err := c.post("/api/v1/book", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// BookUpdate updates the cart (delivery mode, insurances, etc.).
// groupID comes from GET /carts response.
func (c *ConnectClient) BookUpdate(groupID string) error {
	type updateReq struct {
		Prereserve                    bool              `json:"prereserve"`
		Insurances                    []any             `json:"insurances"`
		SelectedDeliveryModeByGroupID map[string]string `json:"selectedDeliveryModeByGroupId"`
		Donations                     []any             `json:"donations"`
		IsForceChoiceTouched          bool              `json:"isForceChoiceTouched"`
	}
	resp, err := c.post("/api/v1/book/update", updateReq{
		Prereserve:                    false,
		Insurances:                    []any{},
		SelectedDeliveryModeByGroupID: map[string]string{groupID: "TKD"},
		Donations:                     []any{},
		IsForceChoiceTouched:          true,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// FinalizationCreate sets up the payment (does NOT charge).
func (c *ConnectClient) FinalizationCreate(groupID, tripID string, buyer, traveler map[string]string) error {
	type deliveryMode struct {
		GroupID         string `json:"groupId"`
		DeliveryMode    string `json:"deliveryMode"`
		AddressRequired bool   `json:"addressRequired"`
	}
	type travelerEntry struct {
		TripID    string           `json:"tripId"`
		Travelers []map[string]any `json:"travelers"`
	}
	type createReq struct {
		DeliveryModes     []deliveryMode  `json:"deliveryModes"`
		Travelers         []travelerEntry `json:"travelers"`
		Buyer             map[string]string `json:"buyer"`
		Insurances        []any           `json:"insurances"`
		Donations         []any           `json:"donations"`
	}

	trav := map[string]any{
		"civility":    traveler["civility"],
		"dateOfBirth": traveler["birthDate"],
		"firstName":   traveler["firstName"],
		"lastName":    traveler["lastName"],
		"email":       traveler["email"],
		"phoneNumber": traveler["phone"],
		"id":          "0",
	}
	if dc := traveler["discountCardNumber"]; dc != "" {
		trav["discountCard"] = map[string]string{"number": dc}
	}

	resp, err := c.post("/api/v1/finalizations/create", createReq{
		DeliveryModes: []deliveryMode{{GroupID: groupID, DeliveryMode: "TKD", AddressRequired: false}},
		Travelers:     []travelerEntry{{TripID: tripID, Travelers: []map[string]any{trav}}},
		Buyer:         buyer,
		Insurances:    []any{},
		Donations:     []any{},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// GetFinalization returns the payment session (SDK info, amount, eligible means).
func (c *ConnectClient) GetFinalization() ([]byte, error) {
	resp, err := c.get("/api/v1/finalizations")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// MoreItineraries loads the next or previous page of itinerary proposals.
func (c *ConnectClient) MoreItineraries(itineraryID string, next bool) ([]Proposal, error) {
	type moreReq struct {
		ItineraryID    string         `json:"itineraryId"`
		Next           bool           `json:"next"`
		Outward        bool           `json:"outward"`
		TrainExpected  bool           `json:"trainExpected"`
		DecisionSource string         `json:"decisionSource"`
		MetadataY      map[string]any `json:"metadataY"`
	}
	resp, err := c.postWithDD("/api/v1/itineraries/more", moreReq{
		ItineraryID:    itineraryID,
		Next:           next,
		Outward:        true,
		TrainExpected:  true,
		DecisionSource: "FORCE",
		MetadataY:      map[string]any{},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}

	type rawOffer struct {
		ID     string `json:"id"`
		Price  string `json:"priceLabel"`
		Header struct {
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"header"`
	}
	type rawComfort struct {
		Offers []rawOffer `json:"offers"`
	}
	type rawProposal struct {
		ID        string `json:"id"`
		TravelID  string `json:"travelId"`
		Departure struct {
			Station string `json:"originStationLabel"`
			Time    string `json:"timeLabel"`
		} `json:"departure"`
		Arrival struct {
			Station string `json:"destinationStationLabel"`
			Time    string `json:"timeLabel"`
		} `json:"arrival"`
		Duration    string `json:"durationLabel"`
		Transporter string `json:"transporterDescription"`
		BestPrice   string `json:"bestPriceLabel"`
		Status      struct {
			IsBookable bool `json:"isBookable"`
		} `json:"status"`
		Second *rawComfort `json:"secondComfortClassOffers"`
		First  *rawComfort `json:"firstComfortClassOffers"`
	}
	type proposalsWrap struct {
		Proposals []rawProposal `json:"proposals"`
	}
	var r struct {
		LongDistance *struct {
			Proposals *proposalsWrap `json:"proposals"`
		} `json:"longDistance"`
	}
	if err := resp.Body.JSON(&r); err != nil {
		return nil, fmt.Errorf("failed to parse more itineraries: %w", err)
	}
	if r.LongDistance == nil || r.LongDistance.Proposals == nil {
		return nil, nil
	}

	var out []Proposal
	for _, p := range r.LongDistance.Proposals.Proposals {
		prop := Proposal{
			ID: p.ID, TravelID: p.TravelID, ItineraryID: itineraryID,
			Departure: p.Departure.Time, Arrival: p.Arrival.Time,
			Duration: p.Duration, Origin: p.Departure.Station,
			Destination: p.Arrival.Station, Transporter: p.Transporter,
			BestPrice: p.BestPrice, IsBookable: p.Status.IsBookable,
		}
		appendOffers := func(src *rawComfort, class string) {
			if src == nil {
				return
			}
			for _, o := range src.Offers {
				prop.Offers = append(prop.Offers, Offer{
					ID: o.ID, Price: o.Price, FareName: o.Header.Title, Class: class,
				})
			}
		}
		appendOffers(p.Second, "2nd")
		appendOffers(p.First, "1st")
		out = append(out, prop)
	}
	return out, nil
}

// GetVehicleDetails returns the train composition, stops, and occupancy.
func (c *ConnectClient) GetVehicleDetails(trainNumber, date, originID, destID string) ([]byte, error) {
	type req struct {
		Number      string `json:"number"`
		Date        string `json:"date"`
		Origin      string `json:"origin"`
		Destination string `json:"destination"`
	}
	resp, err := c.post("/api/v1/vehicle/detail", req{
		Number: trainNumber, Date: date, Origin: originID, Destination: destID,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// UniversalSearch performs a global search (stations, products, help).
func (c *ConnectClient) UniversalSearch(text string) ([]byte, error) {
	type req struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	resp, err := c.post("/api/v1/search", req{ID: uuid4(), Text: text})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// Checkin performs online check-in for a trip.
func (c *ConnectClient) Checkin(tripID string, travelers []map[string]string) error {
	type req struct {
		Travelers []map[string]string `json:"travelers"`
	}
	resp, err := c.post("/api/v3/trip/"+tripID+"/checkin", req{Travelers: travelers})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// GetAppleWalletPasses returns Apple Wallet passes for a trip.
func (c *ConnectClient) GetAppleWalletPasses(tripID string) ([]byte, error) {
	resp, err := c.get("/api/v3/trips/" + tripID + "/apple-wallet-passes")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetGoogleWalletPass returns the Google Wallet pass for a trip.
func (c *ConnectClient) GetGoogleWalletPass(tripID string) ([]byte, error) {
	resp, err := c.get("/api/v3/trips/" + tripID + "/google-wallet-pass")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// InitCancellation starts a cancellation flow for a trip.
func (c *ConnectClient) InitCancellation(tripID, reference string) ([]byte, error) {
	type req struct {
		TripID    string `json:"tripId"`
		Reference string `json:"reference"`
		Origin    string `json:"origin"`
	}
	resp, err := c.post("/api/v1/cancellations", req{
		TripID: tripID, Reference: reference, Origin: "TRIP_DETAIL",
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// QuoteCancellation gets the refund quote for a cancellation.
func (c *ConnectClient) QuoteCancellation(cancellationID string, journeyIDs []string) ([]byte, error) {
	type req struct {
		CancellationID    string   `json:"cancellationId"`
		SelectedJourneyIDs []string `json:"selectedJourneyIds"`
	}
	resp, err := c.post("/api/v1/cancellations/quote", req{
		CancellationID: cancellationID, SelectedJourneyIDs: journeyIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// FinalizeCancellation completes a cancellation.
func (c *ConnectClient) FinalizeCancellation(cancellationID, email string) error {
	type req struct {
		CancellationID string `json:"cancellationId"`
		Email          string `json:"email"`
	}
	resp, err := c.post("/api/v1/cancellations/finalize", req{
		CancellationID: cancellationID, Email: email,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// InitExchange starts an exchange flow for a trip.
func (c *ConnectClient) InitExchange(tripID, reference string) ([]byte, error) {
	type req struct {
		TripID    string `json:"tripId"`
		Reference string `json:"reference"`
		Origin    string `json:"origin"`
	}
	resp, err := c.post("/api/v1/exchanges", req{
		TripID: tripID, Reference: reference, Origin: "TRIP_DETAIL",
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetSeatMap returns the seat map for a specific offer segment.
func (c *ConnectClient) GetSeatMap(itineraryID, offerID, segmentID string) ([]byte, error) {
	path := fmt.Sprintf("/api/v1/itineraries/%s/offers/%s/segments/%s/seatmap",
		itineraryID, offerID, segmentID)
	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetAvailableSeats returns real-time seat availability for an itinerary.
func (c *ConnectClient) GetAvailableSeats(itineraryID string) ([]byte, error) {
	type req struct {
		AvailableSeatRequestID string `json:"availableSeatRequestId"`
		ItineraryID            string `json:"itineraryId"`
	}
	resp, err := c.post("/api/v1/availableseat", req{
		AvailableSeatRequestID: uuid4(), ItineraryID: itineraryID,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetStationInfo returns detailed station information.
func (c *ConnectClient) GetStationInfo(rrCode string) ([]byte, error) {
	resp, err := c.get("/api/v1/stations/" + rrCode)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetBoardsV3 returns departure/arrival boards (V3, richer than V1).
func (c *ConnectClient) GetBoardsV3(stationID string) ([]byte, error) {
	resp, err := c.get("/api/v3/boards/" + stationID)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetHotNews returns current SNCF news and disruption highlights.
func (c *ConnectClient) GetHotNews() ([]byte, error) {
	resp, err := c.get("/api/v1/edito/hotnews")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetNewFeatures returns recently launched features.
func (c *ConnectClient) GetNewFeatures() ([]byte, error) {
	resp, err := c.get("/api/v1/edito/new-features")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// GetFavoriteItineraries returns saved favorite itineraries.
func (c *ConnectClient) GetFavoriteItineraries() ([]byte, error) {
	resp, err := c.get("/api/v1/home/favorites/itineraries")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// AddFavoriteItinerary saves an itinerary as a favorite.
func (c *ConnectClient) AddFavoriteItinerary(itineraryID, itineraryType string) error {
	type req struct {
		ItineraryID string `json:"itineraryId"`
		Type        string `json:"type"`
	}
	resp, err := c.post("/api/v1/itineraries/favorite", req{
		ItineraryID: itineraryID, Type: itineraryType,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// DeleteFavoriteItinerary removes a favorite itinerary.
func (c *ConnectClient) DeleteFavoriteItinerary(itineraryID string) error {
	resp, err := c.post("/api/v1/itineraries/favorite/"+itineraryID, map[string]any{})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// GetDiscountCardPricing returns pricing for discount cards.
func (c *ConnectClient) GetDiscountCardPricing() ([]byte, error) {
	resp, err := c.get("/api/v1/catalog/pricing/discount-card")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// DeletePaymentCard removes a saved payment card.
func (c *ConnectClient) DeletePaymentCard(cardID string) error {
	type req struct {
		CardID string `json:"cardId"`
	}
	resp, err := c.post("/api/v1/accounts/payment-cards/delete", req{CardID: cardID})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// GenerateProofsForCards generates purchase proofs for discount cards.
func (c *ConnectClient) GenerateProofsForCards(startDate, endDate string) error {
	type req struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	resp, err := c.post("/api/v3/proofs-of-purchase/generate-for-cards", req{
		StartDate: startDate, EndDate: endDate,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// SearchExchangeOutward searches for replacement outward trains.
func (c *ConnectClient) SearchExchangeOutward(exchangeID string, origin, dest string, date string) ([]byte, error) {
	type schedule struct {
		Date      string `json:"date"`
		ArrivalAt bool   `json:"arrivalAt"`
	}
	type req struct {
		ExchangeID      string   `json:"exchangeId"`
		Origin          string   `json:"origin"`
		Destination     string   `json:"destination"`
		OutwardSchedule schedule `json:"outwardSchedule"`
		Travelers       []any    `json:"travelers"`
	}
	resp, err := c.post("/api/v1/exchanges/search/outward", req{
		ExchangeID:      exchangeID,
		Origin:          origin,
		Destination:     dest,
		OutwardSchedule: schedule{Date: date, ArrivalAt: false},
		Travelers:       []any{},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// Close releases the surf client resources.
func (c *ConnectClient) Close() {
	if c.http != nil {
		_ = c.http.Close()
	}
}

// BookTripOption adds a trip option (e.g. 1st class upgrade).
func (c *ConnectClient) BookTripOption(name, reference string) error {
	type optionReq struct {
		Name      string `json:"name"`
		Reference string `json:"reference"`
	}
	resp, err := c.post("/api/v1/book/trip-option", optionReq{Name: name, Reference: reference})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// Prereserve places a pre-reservation option on the current cart.
func (c *ConnectClient) Prereserve(buyer, traveler map[string]string) error {
	type prereserveReq struct {
		Buyer             map[string]string   `json:"buyer"`
		Donations         []any               `json:"donations"`
		GenericNewsletter bool                `json:"genericNewsletter"`
		Insurances        []any               `json:"insurances"`
		Travelers         []map[string]string `json:"travelers"`
	}
	resp, err := c.post("/api/v1/finalizations/prereserve", prereserveReq{
		Buyer: buyer, Donations: []any{}, Insurances: []any{},
		Travelers: []map[string]string{traveler},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// RedeemDigitalVoucher applies a digital voucher code to the finalization.
func (c *ConnectClient) RedeemDigitalVoucher(voucherCode string) error {
	type voucherReq struct {
		DigitalVoucherCode string `json:"digitalVoucherCode"`
	}
	resp, err := c.post("/api/v1/finalizations/redeemDigitalVoucher", voucherReq{DigitalVoucherCode: voucherCode})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// CancelDigitalVoucher removes a digital voucher from the finalization.
func (c *ConnectClient) CancelDigitalVoucher(voucherID string) error {
	type cancelReq struct {
		DigitalVoucherID string `json:"digitalVoucherId"`
	}
	resp, err := c.post("/api/v1/finalizations/cancelDigitalVoucher", cancelReq{DigitalVoucherID: voucherID})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}

// GetSplitPaymentSchedule returns the split payment schedule (Alma).
func (c *ConnectClient) GetSplitPaymentSchedule() ([]byte, error) {
	resp, err := c.get("/api/v1/finalizations/split-payment-schedule")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return []byte(resp.Body.Bytes().Ok()), nil
}

// DeleteCartTravel removes a travel from the cart.
func (c *ConnectClient) DeleteCartTravel(travelID string) error {
	resp, err := c.httpDelete("/api/v1/carts/travels/" + travelID)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return classifyErr(int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return nil
}
