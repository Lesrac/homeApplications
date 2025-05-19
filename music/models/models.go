package models

type Song struct {
	Title string `json:"title"`
}

type Songs struct {
	Songs []Song `json:"songs"`
}
