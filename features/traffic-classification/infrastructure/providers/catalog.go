package providers

import "github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/domain"

func DefaultProviders() []DomainProvider {
	return []DomainProvider{
		NewDomainProvider("youtube", []DomainRule{{
			Service:  "YouTube",
			Category: domain.CategoryVideoStreaming,
			Suffixes: []string{"youtube.com", "youtu.be", "googlevideo.com", "ytimg.com"},
		}}),
		NewDomainProvider("netflix", []DomainRule{{
			Service:  "Netflix",
			Category: domain.CategoryVideoStreaming,
			Suffixes: []string{"netflix.com", "nflxvideo.net", "nflximg.net", "nflxso.net"},
		}}),
		NewDomainProvider("prime_video", []DomainRule{{
			Service:  "Prime Video",
			Category: domain.CategoryVideoStreaming,
			Suffixes: []string{"primevideo.com", "amazonvideo.com", "media-amazon.com", "aiv-cdn.net"},
		}}),
		NewDomainProvider("meta", []DomainRule{
			{Service: "Facebook", Category: domain.CategorySocialMedia, Suffixes: []string{"facebook.com", "fbcdn.net", "facebook.net"}},
			{Service: "Instagram", Category: domain.CategorySocialMedia, Suffixes: []string{"instagram.com", "cdninstagram.com"}},
		}),
		NewDomainProvider("tiktok", []DomainRule{{
			Service:  "TikTok",
			Category: domain.CategorySocialMedia,
			Suffixes: []string{"tiktok.com", "tiktokcdn.com", "tiktokv.com"},
		}}),
		NewDomainProvider("x", []DomainRule{{
			Service:  "X",
			Category: domain.CategorySocialMedia,
			Suffixes: []string{"x.com", "twitter.com", "twimg.com"},
		}}),
		NewDomainProvider("snapchat", []DomainRule{{
			Service:  "Snapchat",
			Category: domain.CategorySocialMedia,
			Suffixes: []string{"snapchat.com", "sc-cdn.net"},
		}}),
		NewDomainProvider("downloads", []DomainRule{
			{Service: "BitTorrent", Category: domain.CategoryDownloads, Suffixes: []string{"tracker.opentrackr.org", "openbittorrent.com"}},
			{Service: "Direct Download", Category: domain.CategoryDownloads, Suffixes: []string{"github-releases.githubusercontent.com", "sourceforge.net"}},
		}),
		NewDomainProvider("software_updates", []DomainRule{
			{Service: "Windows Update", Category: domain.CategorySoftwareUpdate, Suffixes: []string{"windowsupdate.com", "update.microsoft.com", "delivery.mp.microsoft.com"}},
			{Service: "Google/Android Update", Category: domain.CategorySoftwareUpdate, Suffixes: []string{"android.clients.google.com", "dl.google.com", "edgedl.me.gvt1.com"}},
		}),
	}
}
