package atomfeed

import (
	"testing"
	"time"
)

func TestGen(t *testing.T) {
	feed := &Feed{Title: "My little feed +=<>&;._-", Link: "http://blog.kowalczyk.info/fofou/feedrss", Description: "Forum for the unaware <>&;"}
	items := make([]*FeedItem, 0)
	i := &FeedItem{Title: "Item 1", Link: &Link{Href:"http://blog.kowalczyk.info/item/1.html"}, Description: "Item 1 description <>;", PubDate: time.Now()}
	items = append(items, i)
	feed.Items = items
	data, err := GenerateAtomFeed(feed)
	if err != nil {
		t.Fatalf("GenerateAtomFeed() returned error %s", err.Error())
	}
	s := string(data)
	if s != `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>My little feed +=&lt;&gt;&amp;;._-</title><link href="http://blog.kowalczyk.info/fofou/feedrss" rel="alternate"></link><id>http://blog.kowalczyk.info/fofou/feedrss</id><updated>2012-09-11T07:39:41Z</updated><entry><title>Item 1</title><link href="http://blog.kowalczyk.info/item/1.html" rel="alternate"></link><updated>2012-09-11T07:39:41Z</updated><id>tag:blog.kowalczyk.info,2012-09-11:/item/1.html</id><summary type="html">Item 1 description &lt;&gt;;</summary></entry></feed>` {
		t.Errorf("wrong result: %s", s)
	}
}
