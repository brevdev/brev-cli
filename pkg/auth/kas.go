package auth

var _ OAuth = KasAuthenticator{}

type KasAuthenticator struct {
	Audience           string
	ClientID           string
	DeviceCodeEndpoint string
	OauthTokenEndpoint string
}
