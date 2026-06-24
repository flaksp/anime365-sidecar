package anilistclient

type GetMediaResponse struct {
	Media *Media `json:"Media"`
}

type Media struct {
	BannerImage *string `json:"bannerImage"`
}
