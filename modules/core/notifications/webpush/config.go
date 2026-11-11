package webpush

import (
	"errors"
	"strings"
	"time"
)

// Environment variables read by FromEnv.
const (
	EnvPublicKey  = "WEB_PUSH_VAPID_PUBLIC_KEY"
	EnvPrivateKey = "WEB_PUSH_VAPID_PRIVATE_KEY"
	EnvSubject    = "WEB_PUSH_SUBJECT"
	EnvHosts      = "WEB_PUSH_ALLOWED_HOSTS"
	EnvTTL        = "WEB_PUSH_TTL"
)

// FromEnv reads the adapter options through lookup (os.LookupEnv in an
// application). ok is false when none of the key variables is set: web push
// stays off unless the deployment configures it. A partial
// configuration is an error so a missing secret is noticed at startup.
//
//	WEB_PUSH_VAPID_PUBLIC_KEY   base64url public key (from GenerateKeys)
//	WEB_PUSH_VAPID_PRIVATE_KEY  base64url private key; a secret
//	WEB_PUSH_SUBJECT            mailto: or https: contact URL
//	WEB_PUSH_ALLOWED_HOSTS      optional comma-separated host patterns (default: the major browsers' push services)
//	WEB_PUSH_TTL                optional Go duration, for example 12h
func FromEnv(lookup func(string) (string, bool)) (Options, bool, error) {
	get := func(name string) string {
		v, _ := lookup(name)
		return strings.TrimSpace(v)
	}
	public, private, subject := get(EnvPublicKey), get(EnvPrivateKey), get(EnvSubject)
	if public == "" && private == "" && subject == "" {
		return Options{}, false, nil
	}
	if public == "" || private == "" || subject == "" {
		return Options{}, false, errors.New("webpush: " + EnvPublicKey + ", " + EnvPrivateKey + " and " + EnvSubject + " must be set together")
	}
	opts := Options{PublicKey: public, PrivateKey: private, Subject: subject}
	if hosts := get(EnvHosts); hosts != "" {
		for _, h := range strings.Split(hosts, ",") {
			if h = strings.TrimSpace(h); h != "" {
				opts.AllowedHosts = append(opts.AllowedHosts, h)
			}
		}
	}
	if ttl := get(EnvTTL); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return Options{}, false, errors.New("webpush: " + EnvTTL + " is not a duration")
		}
		opts.TTL = d
	}
	return opts, true, nil
}
