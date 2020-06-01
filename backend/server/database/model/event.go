package dbmodel

import (
	"time"

	"github.com/go-pg/pg/v9"
	//"github.com/go-pg/pg/v9/orm"
	"github.com/pkg/errors"
	//dbops "isc.org/stork/server/database"
)

// Event levels.
const (
	EvInfo = 0 // informational
	EvWarn = 1 // someone should look into this
	EvErro = 2 // there is a serious problem
)

// Relations between the event and other entities.
type Relations struct {
	Machine int64 `json:",omitempty"`
	App     int64 `json:",omitempty"`
	Subnet  int64 `json:",omitempty"`
	Daemon  int64 `json:",omitempty"`
}

// Represents an event held in event table in the database.
type Event struct {
	ID        int64
	CreatedAt time.Time
	Text      string
	Level     int `pg:",use_zero"`
	Relations *Relations
}

// Add given event to the database.
func AddEvent(db *pg.DB, event *Event) error {
	err := db.Insert(event)
	if err != nil {
		err = errors.Wrapf(err, "problem with inserting event %+v", event)
	}
	return err
}

// Fetches a collection of events from the database. The offset and
// limit specify the beginning of the page and the maximum size of the
// page. Limit has to be greater then 0, otherwise error is
// returned. sortField allows indicating sort column in database and
// sortDir allows selection the order of sorting. If sortField is
// empty then id is used for sorting. If SortDirAny is used then ASC
// order is used.
func GetEventsByPage(db *pg.DB, offset int64, limit int64, sortField string, sortDir SortDirEnum) ([]Event, int64, error) {
	if limit == 0 {
		return nil, 0, errors.New("limit should be greater than 0")
	}
	var events []Event

	// prepare query
	q := db.Model(&events)

	// prepare sorting expression, offset and limit
	ordExpr := prepareOrderExpr("event", sortField, sortDir)
	q = q.OrderExpr(ordExpr)
	q = q.Offset(int(offset))
	q = q.Limit(int(limit))

	total, err := q.SelectAndCount()
	if err != nil {
		return nil, 0, errors.Wrapf(err, "problem with getting events")
	}
	return events, int64(total), nil
}
