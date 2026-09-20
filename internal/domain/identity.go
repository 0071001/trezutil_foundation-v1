package domain

type Identity struct {
	ID              string
	UserID          string
	Provider        string
	ProviderSubject string
	Metadata        map[string]string
	CreatedAt       int64
}

type Provider string

const (
	ProviderWhatsApp Provider = "whatsapp"
	ProviderGoogle   Provider = "google"
	ProviderApple    Provider = "apple"
)

func NewIdentity(userID, provider, providerSubject string) *Identity {
	return &Identity{
		ID:              "",
		UserID:          userID,
		Provider:        provider,
		ProviderSubject: providerSubject,
		Metadata:        make(map[string]string),
		CreatedAt:       0,
	}
}
