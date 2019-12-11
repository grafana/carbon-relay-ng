package queue

// This has been extracted from goque, to tune the queue settings

import (
	"errors"
	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"os"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var ErrDBClosed = errors.New("DB is closed")
var ErrEmpty = errors.New("empty")
var ErrOutOfBounds = errors.New("out of bonds")

type Queue struct {
	sync.RWMutex
	DataDir string
	db      *leveldb.DB
	head    uint64
	tail    uint64
	isOpen  bool
}

func OpenQueue(dataDir string, o *opt.Options) (*Queue, error) {
	var err error

	// Create a new Queue.
	q := &Queue{
		DataDir: dataDir,
		db:      &leveldb.DB{},
		head:    0,
		tail:    0,
		isOpen:  false,
	}

	// Open database for the queue.
	q.db, err = leveldb.OpenFile(dataDir, nil)
	if err != nil {
		return q, err
	}

	q.isOpen = true
	return q, q.init()
}

// Enqueue adds an item to the queue.
func (q *Queue) Enqueue(value []byte, tags encoding.Tags) (*Item, error) {
	q.Lock()
	defer q.Unlock()

	// Check if queue is closed.
	if !q.isOpen {
		return nil, ErrDBClosed
	}

	// Create new Item.
	item := &Item{
		ID:    q.tail + 1,
		Key:   encodeID(q.tail + 1),
		Value: value,
		Tags:  tags,
	}

	// Add it to the queue.
	if err := q.db.Put(item.Key, item.Value, nil); err != nil {
		return nil, err
	}

	// Increment tail position.
	q.tail++

	return item, nil
}

// Dequeue removes the next item in the queue and returns it.
func (q *Queue) Dequeue() (*Item, error) {
	q.Lock()
	defer q.Unlock()

	// Check if queue is closed.
	if !q.isOpen {
		return nil, ErrDBClosed
	}

	// Try to get the next item in the queue.
	item, err := q.getItemByID(q.head + 1)
	if err != nil {
		return nil, err
	}

	// Remove this item from the queue.
	if err := q.db.Delete(item.Key, nil); err != nil {
		return nil, err
	}

	// Increment head position.
	q.head++

	return item, nil
}

// Length returns the total number of items in the queue.
func (q *Queue) Length() uint64 {
	return q.tail - q.head
}

// Close closes the LevelDB database of the queue.
func (q *Queue) Close() {
	q.Lock()
	defer q.Unlock()

	// Check if queue is already closed.
	if !q.isOpen {
		return
	}

	// Reset queue head and tail.
	q.head = 0
	q.tail = 0

	q.db.Close()
	q.isOpen = false
}

// Drop closes and deletes the LevelDB database of the queue.
func (q *Queue) Drop() {
	q.Close()
	os.RemoveAll(q.DataDir)
}

// getItemByID returns an item, if found, for the given ID.
func (q *Queue) getItemByID(id uint64) (*Item, error) {
	// Check if empty or out of bounds.
	if q.Length() == 0 {
		return nil, ErrEmpty
	} else if id <= q.head || id > q.tail {
		return nil, ErrOutOfBounds
	}

	// Get item from database.
	var err error
	item := &Item{ID: id, Key: encodeID(id)}
	if item.Value, err = q.db.Get(item.Key, nil); err != nil {
		return nil, err
	}

	return item, nil
}

// init initializes the queue data.
func (q *Queue) init() error {
	// Create a new LevelDB Iterator.
	iter := q.db.NewIterator(nil, nil)
	defer iter.Release()

	// Set queue head to the first item.
	if iter.First() {
		q.head = keyToID(iter.Key()) - 1
	}

	// Set queue tail to the last item.
	if iter.Last() {
		q.tail = keyToID(iter.Key())
	}

	return iter.Error()
}
