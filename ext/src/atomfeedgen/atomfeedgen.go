package atomfeedgen

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

func GenerateAtomFeed(feed *Feed) string {
	return ""
}
