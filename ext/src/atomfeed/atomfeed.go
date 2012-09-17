package atomfeed

import (
	"time"
    "encoding/xml"
)

// Generates Atom feed

const AtomFeedNamespace = "http://www.w3.org/2005/Atom"

type itemXml struct {
    Title       string `xml:"title"`
    Link        *Link
    Description string
    PubDate     time.Time
}

type  items []itemXml

type feedXml struct {
    XMLName     xml.Name `xml:"feed"`
    Title       string `xml:"title"`
    Link        string `xml:"link"`
    Ns          string `xml:"xmlns,attr"`
    Id          string `xml:"id"`
}

type Feed struct {
	Title       string
	Link        string
	Description string

	items       []*FeedItem
}

type Link struct {
    XMLName     xml.Name `xml:"link"`
    Href string `xml:"href,attr"`
}

type FeedItem struct {
    Title string
    Link string
    Description string
    PubDate time.Time
}

func (f *Feed) AddItem(item *FeedItem) {
    if f.items == nil {
        f.items = make([]*FeedItem,0)
    }
    f.items = append(f.items, item)
}

func (f *Feed) GenXml() ([]byte, error) {
    return nil, nil
}

/*
    feed = feedgenerator.Atom1Feed(
      title = forum.title or forum.url,
      link = my_hostname() + siteroot + "rss",
      description = forum.tagline)
  
    topics = Topic.gql("WHERE forum = :1 AND is_deleted = False ORDER BY created_on DESC", forum).fetch(25)
    for topic in topics:
      title = topic.subject
      link = my_hostname() + siteroot + "topic?id=" + str(topic.key().id())
      first_post = Post.gql("WHERE topic = :1 ORDER BY created_on", topic).get()
      msg = first_post.message
      # TODO: a hack: using a full template to format message body.
      # There must be a way to do it using straight django APIs
      name = topic.created_by
      if name:
        t = Template("<strong>{{ name }}</strong>: {{ msg|striptags|escape|urlize|linebreaksbr }}")
      else:
        t = Template("{{ msg|striptags|escape|urlize|linebreaksbr }}")
      c = Context({"msg": msg, "name" : name})
      description = t.render(c)
      pubdate = topic.created_on
      feed.add_item(title=title, link=link, description=description, pubdate=pubdate)
*/

/*
<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>My little feed +=&lt;&gt;&amp;;._-</title>
  <link href="http://blog.kowalczyk.info/fofou/feedrss" rel="alternate"></link>
  <id>http://blog.kowalczyk.info/fofou/feedrss</id>
  <updated>2012-09-11T07:39:41Z</updated>
  <entry>
    <title>Item 1</title>
    <link href="http://blog.kowalczyk.info/item/1.html" rel="alternate"></link>
    <updated>2012-09-11T07:39:41Z</updated>
    <id>tag:blog.kowalczyk.info,2012-09-11:/item/1.html</id>
    <summary type="html">Item 1 description &lt;&gt;;</summary>
    </entry>
</feed>
*/

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

func GenerateAtomFeed(feed *Feed) ([]byte, error) {
    //return xml.Marshal(feed)
    feed.Ns = AtomFeedNamespace
    feed.test = "Test value"
    return xml.MarshalIndent(feed, "", "  ")
}
