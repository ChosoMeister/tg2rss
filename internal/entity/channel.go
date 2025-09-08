package entity

import "time"

type Channel struct {
	Username string
	Title    string
	URL      string
	ImageURL string
	Posts    []Post
}

type Post struct {
	// Post ID, e.g. 123
	ID          int
	URL         string
	Title       string
	ContentHTML string
	// True if this post contains only image(s) without text content
	ImageOnly bool
	// A preview image that goes to enclosure
	Preview *Image
	// Collection of all images in the post
	Images []Image
	// Date and time of the post in RFC3339 format.
	Datetime time.Time
}

// Image represents an image attachment with its metadata
type Image struct {
	URL  string
	Type string
	// In bytes
	Size int64
}
