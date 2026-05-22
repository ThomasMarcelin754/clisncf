package api

// Journey represents a train journey search result.
type Journey struct {
	DepartureTime string `json:"departure_time"`
	ArrivalTime   string `json:"arrival_time"`
	Duration      string `json:"duration"`
	TrainNumber   string `json:"train_number"`
	TrainType     string `json:"train_type"`
	Status        string `json:"status"`
	CO2           string `json:"co2,omitempty"`
}

// TrainStatus represents real-time status of a train.
type TrainStatus struct {
	TrainNumber string       `json:"train_number"`
	TrainType   string       `json:"train_type"`
	Status      string       `json:"status"`
	Stops       []StopStatus `json:"stops"`
}

// StopStatus represents a single stop in a train's journey.
type StopStatus struct {
	StationName   string `json:"station_name"`
	ArrivalTime   string `json:"arrival_time"`
	DepartureTime string `json:"departure_time"`
	Delay         string `json:"delay"`
}

// Trip represents a booked trip/reservation.
type Trip struct {
	Date        string `json:"date"`
	From        string `json:"from"`
	To          string `json:"to"`
	TrainNumber string `json:"train_number"`
	BookingRef  string `json:"booking_ref"`
	Status      string `json:"status"`
}

// CartItem is one travel currently in the basket.
type CartItem struct {
	Title      string `json:"title"`
	From       string `json:"from"`
	To         string `json:"to"`
	When       string `json:"when"`
	PriceLabel string `json:"price_label"`
}

// Cart is the current basket (GET /bff/api/v1/carts) — read-only.
type Cart struct {
	Items      []CartItem `json:"items"`
	TotalLabel string     `json:"total_label"`
	TotalValue int        `json:"total_value"`
	Currency   string     `json:"currency"`
	NextStep   string     `json:"next_step"`
}

// Proof is one purchasable-justificatif trip from GET /api/v1/proofs-of-purchase.
type Proof struct {
	Month     string `json:"month"`
	Date      string `json:"date"`
	From      string `json:"from"`
	To        string `json:"to"`
	Route     string `json:"route"`
	Reference string `json:"reference"`
}

// AccountDisplay is the account from GET /accounts/display → display.account.
type AccountDisplay struct {
	AccountID    string       `json:"accountId"`
	PersonalData PersonalData `json:"personalData"`
	DiscountCards []DiscountCard `json:"discountCards"`
	PaymentCards  []PaymentCard  `json:"paymentCards"`
	Companions    []Companion    `json:"companions"`
	CompanyName   string         `json:"companyName"`
	EmailPro      string         `json:"emailPro"`
	IUC           string         `json:"iuc"`
}

type PersonalData struct {
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Civility  string `json:"civility"`
	BirthDate string `json:"birthdate"`
	Email     string `json:"email"`
}

type DiscountCard struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Number    string `json:"number"`
	ExpiresAt string `json:"expirationDate"`
}

// CardCode is the loyalty card QR from GET /accounts/card-code.
type CardCode struct {
	QRCode string `json:"qrCode"`
}

// Companion from display.account.companions[].
type Companion struct {
	ID             string       `json:"id"`
	PersonalData   PersonalData `json:"personalData"`
	TravellerKind  string       `json:"travellerKind"`
	DiscountCards  []DiscountCard `json:"discountCards"`
}

// PaymentCard from display.account.paymentCards[].
type PaymentCard struct {
	ID             string `json:"id"`
	MaskedNumber   string `json:"maskedNumber"`
	IsMain         bool   `json:"isMain"`
	Type           string `json:"type"`
	ExpirationDate string `json:"expirationDate"`
	Label          string `json:"label"`
}

// Alert types for GET /api/v1/alerting/*.
type LowPriceAlert struct {
	ID          string `json:"id"`
	Origin      string `json:"originLabel"`
	Destination string `json:"destinationLabel"`
	OutwardDate string `json:"outwardDate"`
	MaxPrice    string `json:"maxPriceLabel"`
}

type SalesOpeningAlert struct {
	ID          string `json:"id"`
	Origin      string `json:"originLabel"`
	Destination string `json:"destinationLabel"`
	Date        string `json:"dateLabel"`
}

type FullTrainAlert struct {
	ID            string `json:"id"`
	Origin        string `json:"originLabel"`
	Destination   string `json:"destinationLabel"`
	DepartureDate string `json:"departureDateLabel"`
}

// LowPriceODEligibility from POST /api/v1/alerting/low-price-alert/check/od.
type LowPriceODEligibility struct {
	Eligible   bool           `json:"eligible"`
	DaysOfWeek []DayOfWeek    `json:"daysOfWeek"`
	TimeSlots  []TimeSlotHour `json:"timeSlots"`
}

type DayOfWeek struct {
	Value string `json:"value"`
	Title struct {
		Label         string `json:"label"`
		SemanticLabel string `json:"semanticLabel"`
	} `json:"title"`
}

type TimeSlotHour struct {
	Hour  int    `json:"hour"`
	Label string `json:"label"`
}

// LowPriceScheduleEligibility from POST /api/v1/alerting/low-price-alert/check/schedule.
type LowPriceScheduleEligibility struct {
	Eligible              bool            `json:"eligible"`
	InvalidDateMessage    string          `json:"invalidDateMessage"`
	MinStartDateSelectable string         `json:"minStartDateSelectable"`
	MinEndDateSelectable  string          `json:"minEndDateSelectable"`
	MaxDateSelectable     string          `json:"maxDateSelectable"`
	OptIn                 string          `json:"optIn"`
	PriceRange            AlertPriceRange `json:"priceRange"`
}

type AlertPriceRange struct {
	Min            float64 `json:"min"`
	Max            float64 `json:"max"`
	Average        float64 `json:"average"`
	CurrencySymbol string  `json:"currencySymbol"`
}

// BestPriceDay from POST /api/v1/edito/calendar.
type BestPriceDay struct {
	Date      string `json:"date"`
	Price     string `json:"price"`
	Highlight bool   `json:"highlight"`
}

// FavoritePlace from GET /api/v1/favorite-places.
type FavoritePlace struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

// TrafficDisruption from GET /api/v1/trafficinfo.
type TrafficDisruption struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Type    string `json:"type"`
}

// TripsFilter from GET /api/v3/trips/filter.
type TripsFilter struct {
	Label string `json:"label"`
	Page  struct {
		Title       string `json:"title"`
		RadioGroup  struct {
			DefaultValue string `json:"defaultValue"`
			RadioButtons []struct {
				Value string `json:"value"`
				Label string `json:"label"`
			} `json:"radioButtons"`
		} `json:"radioGroup"`
	} `json:"page"`
}

// OptInsPage from GET /api/v1/accounts/optIns → page.
type OptInsPage struct {
	Title      string `json:"title"`
	OptInBlocks []struct {
		Title  string `json:"title"`
		OptIns []OptIn `json:"optIns"`
	} `json:"optInsBlock"`
}

type OptIn struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	IsActive bool   `json:"isActive"`
}

// OfferCategory from GET /api/v1/accounts/offers/settings → categories[].
type OfferCategory struct {
	Title string      `json:"title"`
	Type  string      `json:"type"`
	Items []OfferItem `json:"items"`
}

type OfferItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	IsEnabled bool   `json:"isEnabled"`
}

// BoardsResult from GET /api/v3/boards/{stationId}.
type BoardsResult struct {
	Station struct {
		Label string `json:"label"`
	} `json:"station"`
	BoardsByLineID map[string]LineBoard `json:"boardsByLineId"`
}

type LineBoard struct {
	MainLineBoards *MainLineBoards `json:"mainLineBoards"`
}

type MainLineBoards struct {
	DeparturesBoard struct {
		Items []BoardItem `json:"items"`
	} `json:"departuresBoard"`
	ArrivalsBoard struct {
		Items []BoardItem `json:"items"`
	} `json:"arrivalsBoard"`
}

type BoardItem struct {
	TimeLabel          string `json:"timeLabel"`
	TimeLabelDisrupted string `json:"timeLabelDisrupted"`
	VehicleInfo        struct {
		Label      string `json:"label"`
		ShortLabel string `json:"shortLabel"`
	} `json:"vehicleInfo"`
	DestinationLabel string `json:"destinationLabel"`
	OriginLabel      string `json:"originLabel"`
	PlatformLabel    string `json:"platformLabel"`
}

// CatalogNode from GET /api/v1/catalog → nodes{}.
type CatalogNode struct {
	Type   string `json:"type"`
	Header struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"header"`
	Body struct {
		Cards []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Price       string `json:"price"`
		} `json:"cards"`
	} `json:"body"`
}

// Proposal is one itinerary proposal from POST /bff/api/v1/itineraries.
type Proposal struct {
	ID          string  `json:"id"`
	TravelID    string  `json:"travel_id"`
	ItineraryID string  `json:"itinerary_id"`
	Departure   string  `json:"departure"`
	Arrival     string  `json:"arrival"`
	Duration    string  `json:"duration"`
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
	Transporter string  `json:"transporter"`
	BestPrice   string  `json:"best_price"`
	IsBookable  bool    `json:"is_bookable"`
	Offers      []Offer `json:"offers,omitempty"`
}

// Offer is a fare option within a Proposal.
type Offer struct {
	ID        string `json:"id"`
	Price     string `json:"price"`
	FareName  string `json:"fare_name"`
	Class     string `json:"class"`
	SegmentID string `json:"segment_id,omitempty"`
}
