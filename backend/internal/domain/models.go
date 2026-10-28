// Package domain holds the CRM entities and the pure business rules that
// operate on them (conflicts, pricing, permissions, availability, signage).
// Nothing here touches the database or the network.
package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin       Role = "admin"
	RoleCoordinator Role = "coordinator"
	RoleStaff       Role = "staff"
	RoleUser        Role = "user"
)

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleCoordinator, RoleStaff, RoleUser:
		return true
	}
	return false
}

type User struct {
	ID             uuid.UUID `json:"id"`
	FirebaseUID    *string   `json:"-"`
	Email          string    `json:"email"`
	FullName       string    `json:"fullName"`
	Phone          *string   `json:"phone"`
	Role           Role      `json:"role"`
	StaffKind      *string   `json:"staffKind"`
	TelegramChatID *string   `json:"telegramChatId"`
	AvatarColor    string    `json:"avatarColor"`
	IsActive       bool      `json:"isActive"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Customer struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	ContactName *string    `json:"contactName"`
	Phone       *string    `json:"phone"`
	Email       *string    `json:"email"`
	TaxID       *string    `json:"taxId"`
	Notes       string     `json:"notes"`
	UserID      *uuid.UUID `json:"userId"`
	CreatedAt   time.Time  `json:"createdAt"`
	// Aggregates (filled by list queries)
	BookingsCount int     `json:"bookingsCount"`
	TotalBilled   float64 `json:"totalBilled"`
	TotalPaid     float64 `json:"totalPaid"`
}

type Building struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Address  string    `json:"address"`
	OpensAt  string    `json:"opensAt"`  // HH:MM
	ClosesAt string    `json:"closesAt"` // HH:MM
}

type Room struct {
	ID            uuid.UUID `json:"id"`
	BuildingID    uuid.UUID `json:"buildingId"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	Capacity      int       `json:"capacity"`
	Color         string    `json:"color"`
	Description   string    `json:"description"`
	Rules         string    `json:"rules"`
	Equipment     []string  `json:"equipment"`
	HourlyRate    float64   `json:"hourlyRate"`
	MinHours      float64   `json:"minHours"`
	BufferMinutes int       `json:"bufferMinutes"`
	IsBookable    bool      `json:"isBookable"`
	IsActive      bool      `json:"isActive"`
	ImageKey      *string   `json:"imageKey"`
	SortOrder     int       `json:"sortOrder"`
}

type EquipmentItem struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	Label     string    `json:"label"`
	FlatPrice float64   `json:"flatPrice"`
}

type Activity struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	CoordinatorID *uuid.UUID `json:"coordinatorId"`
	DefaultRoomID *uuid.UUID `json:"defaultRoomId"`
	Color         string     `json:"color"`
	Audience      string     `json:"audience"`
	IsActive      bool       `json:"isActive"`
	SessionsCount int        `json:"sessionsCount"`
}

type BookingType string

const (
	BookingActivity    BookingType = "activity"
	BookingRental      BookingType = "rental"
	BookingInternal    BookingType = "internal"
	BookingMaintenance BookingType = "maintenance"
	BookingClosed      BookingType = "closed"
)

func (t BookingType) Valid() bool {
	switch t {
	case BookingActivity, BookingRental, BookingInternal, BookingMaintenance, BookingClosed:
		return true
	}
	return false
}

type BookingStatus string

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusCancelled BookingStatus = "cancelled"
	StatusRejected  BookingStatus = "rejected"
)

// Blocks reports whether a booking in this status occupies its room.
func (s BookingStatus) Blocks() bool { return s == StatusPending || s == StatusConfirmed }

type Booking struct {
	ID            uuid.UUID     `json:"id"`
	RoomID        uuid.UUID     `json:"roomId"`
	Title         string        `json:"title"`
	Type          BookingType   `json:"type"`
	Status        BookingStatus `json:"status"`
	StartsAt      time.Time     `json:"startsAt"`
	EndsAt        time.Time     `json:"endsAt"`
	BufferMinutes int           `json:"bufferMinutes"`
	ActivityID    *uuid.UUID    `json:"activityId"`
	CoordinatorID *uuid.UUID    `json:"coordinatorId"`
	CustomerID    *uuid.UUID    `json:"customerId"`
	RequestID     *uuid.UUID    `json:"requestId"`
	SeriesID      *uuid.UUID    `json:"seriesId"`
	Attendees     int           `json:"attendees"`
	Equipment     []string      `json:"equipment"`
	Price         float64       `json:"price"`
	Notes         string        `json:"notes"`
	InternalNotes string        `json:"internalNotes,omitempty"`
	CreatedBy     *uuid.UUID    `json:"createdBy"`
	CreatedAt     time.Time     `json:"createdAt"`
	// Denormalised for display
	RoomName        string    `json:"roomName"`
	BuildingID      uuid.UUID `json:"buildingId"`
	CoordinatorName *string   `json:"coordinatorName"`
	CustomerName    *string   `json:"customerName"`
	PaidAmount      float64   `json:"paidAmount"`
}

func (b Booking) Balance() float64 { return round2(b.Price - b.PaidAmount) }

type RequestStatus string

const (
	RequestPending   RequestStatus = "pending"
	RequestApproved  RequestStatus = "approved"
	RequestRejected  RequestStatus = "rejected"
	RequestCancelled RequestStatus = "cancelled"
)

type BookingRequest struct {
	ID              uuid.UUID     `json:"id"`
	UserID          uuid.UUID     `json:"userId"`
	CustomerID      *uuid.UUID    `json:"customerId"`
	RoomID          uuid.UUID     `json:"roomId"`
	Title           string        `json:"title"`
	EventKind       string        `json:"eventKind"`
	StartsAt        time.Time     `json:"startsAt"`
	EndsAt          time.Time     `json:"endsAt"`
	Attendees       int           `json:"attendees"`
	Equipment       []string      `json:"equipment"`
	Notes           string        `json:"notes"`
	Status          RequestStatus `json:"status"`
	QuotedPrice     float64       `json:"quotedPrice"`
	FinalPrice      *float64      `json:"finalPrice"`
	InternalNotes   string        `json:"internalNotes,omitempty"`
	RejectionReason string        `json:"rejectionReason"`
	ReviewedBy      *uuid.UUID    `json:"reviewedBy"`
	ReviewedAt      *time.Time    `json:"reviewedAt"`
	BookingID       *uuid.UUID    `json:"bookingId"`
	CreatedAt       time.Time     `json:"createdAt"`
	// Denormalised
	RoomName       string  `json:"roomName"`
	RequesterName  string  `json:"requesterName"`
	RequesterEmail string  `json:"requesterEmail"`
	RequesterPhone *string `json:"requesterPhone"`
	// Computed at read time for pending requests
	HasConflict bool `json:"hasConflict"`
}

type PaymentMethod string

const (
	PayBankTransfer PaymentMethod = "bank_transfer"
	PayCheck        PaymentMethod = "check"
	PayCash         PaymentMethod = "cash"
	PayCreditCard   PaymentMethod = "credit_card"
)

func (m PaymentMethod) Valid() bool {
	switch m {
	case PayBankTransfer, PayCheck, PayCash, PayCreditCard:
		return true
	}
	return false
}

type Payment struct {
	ID         uuid.UUID     `json:"id"`
	BookingID  uuid.UUID     `json:"bookingId"`
	Amount     float64       `json:"amount"`
	Method     PaymentMethod `json:"method"`
	Reference  string        `json:"reference"`
	PaidAt     time.Time     `json:"paidAt"`
	Notes      string        `json:"notes"`
	RecordedBy *uuid.UUID    `json:"recordedBy"`
	CreatedAt  time.Time     `json:"createdAt"`
	// Denormalised
	BookingTitle string  `json:"bookingTitle"`
	CustomerName *string `json:"customerName"`
}

type TaskStatus string

const (
	TaskOpen       TaskStatus = "open"
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
	TaskCancelled  TaskStatus = "cancelled"
)

type Task struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Kind           string     `json:"kind"`
	Status         TaskStatus `json:"status"`
	RoomID         *uuid.UUID `json:"roomId"`
	BookingID      *uuid.UUID `json:"bookingId"`
	ScheduleID     *uuid.UUID `json:"scheduleId"`
	AssigneeID     *uuid.UUID `json:"assigneeId"`
	DueAt          time.Time  `json:"dueAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	CompletionNote string     `json:"completionNote"`
	ReminderSentAt *time.Time `json:"reminderSentAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	// Denormalised
	RoomName     *string `json:"roomName"`
	AssigneeName *string `json:"assigneeName"`
}

type TaskSchedule struct {
	ID              uuid.UUID  `json:"id"`
	Title           string     `json:"title"`
	Kind            string     `json:"kind"`
	RoomID          *uuid.UUID `json:"roomId"`
	AssigneeID      *uuid.UUID `json:"assigneeId"`
	Weekdays        []int      `json:"weekdays"`
	TimeOfDay       string     `json:"timeOfDay"` // HH:MM
	DurationMinutes int        `json:"durationMinutes"`
	IsActive        bool       `json:"isActive"`
}

type SignageScreen struct {
	ID            uuid.UUID   `json:"id"`
	Slug          string      `json:"slug"`
	Name          string      `json:"name"`
	BuildingID    *uuid.UUID  `json:"buildingId"`
	Theme         string      `json:"theme"`
	WelcomeText   string      `json:"welcomeText"`
	RotateSeconds int         `json:"rotateSeconds"`
	RoomIDs       []uuid.UUID `json:"roomIds"`
	IsActive      bool        `json:"isActive"`
}

type Announcement struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	ImageKey  *string    `json:"imageKey"`
	StartsAt  time.Time  `json:"startsAt"`
	EndsAt    *time.Time `json:"endsAt"`
	Priority  int        `json:"priority"`
	ScreenID  *uuid.UUID `json:"screenId"`
	CreatedAt time.Time  `json:"createdAt"`
}

type Attachment struct {
	ID          uuid.UUID `json:"id"`
	Entity      string    `json:"entity"`
	EntityID    uuid.UUID `json:"entityId"`
	ObjectKey   string    `json:"-"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Notification struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"userId"`
	Channel   string     `json:"channel"`
	Recipient string     `json:"recipient"`
	Subject   string     `json:"subject"`
	Body      string     `json:"body"`
	Status    string     `json:"status"`
	Error     string     `json:"error,omitempty"`
	ReadAt    *time.Time `json:"readAt"`
	SentAt    *time.Time `json:"sentAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

type AuditEntry struct {
	ID        int64          `json:"id"`
	ActorID   *uuid.UUID     `json:"actorId"`
	ActorName *string        `json:"actorName"`
	Entity    string         `json:"entity"`
	EntityID  *uuid.UUID     `json:"entityId"`
	Action    string         `json:"action"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"createdAt"`
}
