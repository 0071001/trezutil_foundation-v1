package domain

// ConversationResponse represents a canonical response for any channel.
type ConversationResponse struct {
	Text              string
	Language          string
	StructuredContent *StructuredContent
	Cards             []ConversationCard
	Actions           []ConversationActionPrompt
	Confirmation      *ConversationConfirmation
	Metadata          map[string]interface{}
}

// StructuredContent represents structured data in a response.
type StructuredContent struct {
	Type string
	Data map[string]interface{}
}

// ConversationCard represents a card in a response.
type ConversationCard struct {
	ID          string
	Type        string
	Title       string
	Description string
	Data        map[string]interface{}
}

// ConversationActionPrompt represents an actionable prompt in a response.
type ConversationActionPrompt struct {
	ActionType CommerceActionType
	Label      string
	Parameters map[string]interface{}
}

// ConversationConfirmation represents a confirmation request in a response.
type ConversationConfirmation struct {
	ActionID string
	Summary  string
	Details  map[string]interface{}
}

// NewConversationResponse creates a new ConversationResponse.
func NewConversationResponse(text string, language string) *ConversationResponse {
	return &ConversationResponse{
		Text:     text,
		Language: language,
		Metadata: make(map[string]interface{}),
	}
}

// AddCard adds a card to the response.
func (r *ConversationResponse) AddCard(card ConversationCard) {
	r.Cards = append(r.Cards, card)
}

// AddAction adds an action prompt to the response.
func (r *ConversationResponse) AddAction(action CommerceActionType, label string) {
	r.Actions = append(r.Actions, ConversationActionPrompt{
		ActionType: action,
		Label:      label,
	})
}

// SetConfirmation sets confirmation data on the response.
func (r *ConversationResponse) SetConfirmation(confirmation ConversationConfirmation) {
	r.Confirmation = &confirmation
}
