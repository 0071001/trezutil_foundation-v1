package domain

// ConversationResponse represents a canonical response for any channel.
type ConversationResponse struct {
	Text              string                     `json:"text"`
	Language          string                     `json:"language"`
	StructuredContent *StructuredContent         `json:"structured_content,omitempty"`
	Cards             []ConversationCard         `json:"cards,omitempty"`
	Actions           []ConversationActionPrompt `json:"actions,omitempty"`
	Confirmation      *ConversationConfirmation  `json:"confirmation,omitempty"`
	Metadata          map[string]interface{}     `json:"metadata,omitempty"`
}

// StructuredContent represents structured data in a response.
type StructuredContent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// ConversationCard represents a card in a response.
type ConversationCard struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// ConversationActionPrompt represents an actionable prompt in a response.
type ConversationActionPrompt struct {
	ActionType CommerceActionType     `json:"action_type"`
	Label      string                 `json:"label"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ConversationConfirmation represents a confirmation request in a response.
type ConversationConfirmation struct {
	ActionID string                 `json:"action_id"`
	Summary  string                 `json:"summary"`
	Details  map[string]interface{} `json:"details,omitempty"`
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
