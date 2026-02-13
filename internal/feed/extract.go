package feed

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/entity"
)

const (
	maxTitleLength  = 80
	ellipsis        = "…"
	openParenthesis = '('
	punctuation     = ",.;:!? "
)

var (
	breaksRegex         = regexp.MustCompile(`(?:<br\s*/?>\s*){1,}|<p>|</p>`)
	multipleSpacesRegex = regexp.MustCompile(`\s+`)
	sentenceEndRegex    = regexp.MustCompile(`[.!?…](?:\s|$)|\.{3}`)
	imageExtRegex       = regexp.MustCompile(`\.(jpg|jpeg|png|gif)$`)
)

// extractTitle extracts a meaningful title from HTML content following the specified rules.
// It prioritizes:
// 1. First line of text separated by multiple line breaks.
// 2. First sentence or paragraph from the content.
func extractTitle(element *colly.HTMLElement) string {
	msgContainer := findMessageContainer(element)

	if msgContainer == nil {
		return ""
	}

	if title := extractFirstLine(msgContainer); title != "" {
		return formatTitle(title)
	}

	text := msgContainer.Text()
	matches := sentenceEndRegex.FindStringIndex(text)

	if matches != nil {
		return formatTitle(text[:matches[1]])
	}

	return formatTitle(text)
}

// findMessageContainer finds the most appropriate message text container from Telegram HTML.
// This function handles several complex cases:
//
//  1. Messages with replies: When a message quotes/replies to another message, there can be
//     multiple .tgme_widget_message_text elements. The function prioritizes main content
//     over reply content by excluding elements inside .tgme_widget_message_reply containers.
//
//  2. Nested message elements: Sometimes there are multiple nested .tgme_widget_message_text
//     elements within each other. In such cases, the deepest one is preferred.
//
//  3. Multiple main content elements: If there are multiple non-reply message text elements,
//     the function prefers the later ones as main content usually comes after replies.
//
// The function returns nil if no suitable container is found.
func findMessageContainer(element *colly.HTMLElement) *goquery.Selection {
	allMessages := element.DOM.Find(".tgme_widget_message_text")

	if allMessages.Length() == 0 {
		return nil
	}

	// Filter out messages that are inside reply containers to prioritize main content
	var mainIndices []int

	allMessages.Each(func(i int, s *goquery.Selection) {
		// Check if this message text is inside a reply by looking at parents
		isInReply := s.Closest(".tgme_widget_message_reply").Length() > 0
		if !isInReply {
			mainIndices = append(mainIndices, i)
		}
	})

	// Use main messages if found, otherwise use all messages
	var msgContainer *goquery.Selection

	if len(mainIndices) > 0 {
		// Use the first non-reply message
		msgContainer = allMessages.Eq(mainIndices[0])

		// If there are multiple non-reply messages, prefer the later ones (main content usually comes after replies)
		if len(mainIndices) > 1 {
			msgContainer = allMessages.Eq(mainIndices[len(mainIndices)-1])
		}
	} else {
		msgContainer = allMessages
	}

	// Sometimes there are two inner div.tgme_widget_message_text elements
	// nested in each other, in which case the most deep one is used.
	if msgContainer.Length() > 1 {
		deepest := msgContainer

		for {
			nestedElement := deepest.Find(".tgme_widget_message_text")

			if nestedElement.Length() == 0 {
				break
			}

			deepest = nestedElement
		}

		msgContainer = deepest
	}

	return msgContainer
}

// extractFirstLine finds the first line of text before multiple line breaks
func extractFirstLine(selection *goquery.Selection) string {
	html, err := selection.Html()

	if err != nil {
		return ""
	}

	// Split content at multiple line breaks
	parts := breaksRegex.Split(html, 2)

	if len(parts) > 1 {
		// Create a new document from the first part to extract text
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(parts[0]))

		if err != nil {
			return ""
		}

		return strings.TrimSpace(doc.Text())
	}

	return ""
}

// formatTitle ensures the title follows the specified rules
func formatTitle(text string) string {
	// Clean up spaces
	text = multipleSpacesRegex.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Remove parenthetical text if it crosses the character limit
	text = removeIncompleteParens(text, maxTitleLength)

	// Ensure we don't cut words in half
	text = truncateAtWordBoundary(text, maxTitleLength)

	return text
}

// removeIncompleteParens removes parenthetical text that crosses the character limit
func removeIncompleteParens(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}

	var result strings.Builder
	inParens := false
	parenStart := 0
	runeCount := 0

	for i, r := range text {
		runeCount++

		if r == openParenthesis {
			inParens = true
			parenStart = i
		} else if r == ')' {
			inParens = false
		}

		if runeCount > limit && inParens {
			// If we cross the limit while inside parentheses,
			// remove everything from the opening paren
			return strings.TrimRight(text[:parenStart], punctuation) + ellipsis
		}

		if !inParens {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// truncateAtWordBoundary truncates text at a word boundary
func truncateAtWordBoundary(text string, limit int) string {
	// Remove trailing colon if present
	hasColon := strings.HasSuffix(text, ":")

	if hasColon {
		text = strings.TrimSuffix(text, ":")
	}

	runeCount := utf8.RuneCountInString(text)

	// If text is under the limit and had a colon, add ellipsis
	if runeCount <= limit && hasColon {
		return text + ellipsis
	}

	// Otherwise just return the text
	if runeCount <= limit {
		return text
	}

	lastWordEnd := 0
	currentCount := 0

	for i, r := range text {
		currentCount++

		if unicode.IsSpace(r) {
			lastWordEnd = i
		}

		if currentCount >= limit {
			var truncated string

			if lastWordEnd > 0 {
				// Truncate at the last word boundary
				truncated = text[:lastWordEnd]
			} else {
				// If no word boundary found, just truncate at the limit
				truncated = text[:i]
			}

			// Remove trailing punctuation before adding ellipsis
			truncated = strings.TrimRight(truncated, punctuation)

			return truncated + ellipsis
		}
	}

	return text
}

// extractImages gets all images from message grouped layer
func extractImages(element *colly.HTMLElement) []entity.Image {
	var images []entity.Image

	element.ForEach(".tgme_widget_message_photo_wrap", func(_ int, el *colly.HTMLElement) {
		imageURL := extractImageURLFromStyle(el.Attr("style"))

		if imageURL == "" {
			return
		}

		imageType := extractImageTypeFromURL(imageURL)
		imageSize := getImageSize(imageURL)

		images = append(images, entity.Image{
			URL:  imageURL,
			Type: imageType,
			Size: imageSize,
		})
	})

	return images
}

// extractPreview finds an image link preview and extracts it
func extractPreview(element *colly.HTMLElement) *entity.Image {
	previewURL, exists := element.DOM.Find(".tgme_widget_message_link_preview").Attr("href")

	if exists && imageExtRegex.MatchString(previewURL) {
		preview := &entity.Image{
			URL: previewURL,
		}

		preview.Type = extractImageTypeFromURL(preview.URL)
		preview.Size = getImageSize(preview.URL)

		return preview
	}

	return nil
}

func extractImageURLFromStyle(style string) string {
	if style == "" {
		return ""
	}

	urlStart := strings.Index(style, "url(")

	if urlStart == -1 {
		return ""
	}

	urlStart += 4 // Skip "url("
	urlEnd := strings.Index(style[urlStart:], ")") + urlStart

	if urlEnd <= urlStart {
		return ""
	}

	url := style[urlStart:urlEnd]
	url = strings.Trim(url, "'\"")

	return url
}

func extractImageTypeFromURL(url string) string {
	switch filepath.Ext(url) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "" // Skip unsupported image types
	}
}

func getImageSize(imageURL string) int64 {
	logger := app.Logger()

	// nolint: gosec
	res, err := httpClient.Head(imageURL)

	if err != nil {
		logger.Error("Could not get image size",
			"imageUrl", imageURL,
			"error", err)

		return 0
	}

	defer res.Body.Close()

	if res.StatusCode >= 200 && res.StatusCode < 300 && res.ContentLength > 0 {
		return res.ContentLength
	}

	return 0
}
