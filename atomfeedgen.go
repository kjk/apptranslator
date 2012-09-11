package main

import (
	"time"
)

// Generates Atom feed

type Feed struct {
	Title       string
	Link        string
	Description string
	Items       []*FeedItem
}

type FeedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     time.Time
}

/*
def rfc3339_date(date):
    # Support datetime objects older than 1900
    date = datetime_safe.new_datetime(date)
    time_str = date.strftime('%Y-%m-%dT%H:%M:%S')
    if not six.PY3:             # strftime returns a byte string in Python 2
        time_str = time_str.decode('utf-8')
    if is_aware(date):
        offset = date.tzinfo.utcoffset(date)
        timezone = (offset.days * 24 * 60) + (offset.seconds // 60)
        hour, minute = divmod(timezone, 60)
        return time_str + '%+03d:%02d' % (hour, minute)
    else:
        return time_str + 'Z'
*/

func TestAtom() {
	feed := &Feed{Title: "My little feed +=<>&;._-", Link: "http://blog.kowalczyk.info/fofou/feedrss", description: "Forum for the unaware <>&;"}
	items := make([]FeedItem, 0)
	i := &FeedItem{Title: "Item 1", Link: "http://blog.kowalczyk.info/item/1.html", Description: "Item 1 description <>;", PubDate: time.Now()}
	items = append(items, i)
	s := GenerateAtomFeed(feed)
	if s != `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>My little feed +=&lt;&gt;&amp;;._-</title><link href="http://blog.kowalczyk.info/fofou/feedrss" rel="alternate"></link><id>http://blog.kowalczyk.info/fofou/feedrss</id><updated>2012-09-11T07:39:41Z</updated><entry><title>Item 1</title><link href="http://blog.kowalczyk.info/item/1.html" rel="alternate"></link><updated>2012-09-11T07:39:41Z</updated><id>tag:blog.kowalczyk.info,2012-09-11:/item/1.html</id><summary type="html">Item 1 description &lt;&gt;;</summary></entry></feed>` {
		panic("wrong result")
	}
}

func GenerateAtomFeed(feed *Feed) string {
	return ""
}
