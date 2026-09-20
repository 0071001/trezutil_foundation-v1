package domain

// Channel represents a communication channel.
type Channel string

const (
	ChannelWhatsApp Channel = "whatsapp"
	ChannelWeb      Channel = "web"
	ChannelMobile   Channel = "mobile"
	ChannelUSSD     Channel = "ussd"
	ChannelSMS      Channel = "sms"
	ChannelVoice    Channel = "voice"
	ChannelOther    Channel = "other"
)

// IsValid returns true if the channel is recognized.
func (c Channel) IsValid() bool {
	switch c {
	case ChannelWhatsApp, ChannelWeb, ChannelMobile, ChannelUSSD, ChannelSMS, ChannelVoice, ChannelOther:
		return true
	default:
		return false
	}
}

// IsMetaChannel returns true for channels that require provider-specific handling.
func (c Channel) IsMetaChannel() bool {
	return c == ChannelWhatsApp
}
