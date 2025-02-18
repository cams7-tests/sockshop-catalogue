package catalogue

// service.go contains the definition and implementation (business logic) of the
// catalogue service. Everything here is agnostic to the transport (HTTP).

import (
	"fmt"
	"errors"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/jmoiron/sqlx"
)

// Service is the catalogue service, providing read operations on a saleable
// catalogue of sock products.
type Service interface {
	List(tags []string, order string, pageNum, pageSize int) ([]Sock, error) // GET /catalogue
	Count(tags []string) (int, error)                                        // GET /catalogue/size
	Get(id string) (Sock, error)                                             // GET /catalogue/{id}
	Tags() ([]string, error)                                                 // GET /tags
	Health() []Health                                                        // GET /health
}

// Middleware decorates a Service.
type Middleware func(Service) Service

// Sock describes the thing on offer in the catalogue.
type Sock struct {
	ID          string   `json:"id" db:"id"`
	Name        string   `json:"name" db:"name"`
	Description string   `json:"description" db:"description"`
	ImageURL    []string `json:"imageUrl" db:"-"`
	ImageURL_1  string   `json:"-" db:"image_url_1"`
	ImageURL_2  string   `json:"-" db:"image_url_2"`
	Price       float32  `json:"price" db:"price"`
	Count       int      `json:"count" db:"count"`
	Tags        []string `json:"tag" db:"-"`
	TagString   string   `json:"-" db:"tag_name"`
}

// Health describes the health of a service
type Health struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Time    string `json:"time"`
}

// ErrNotFound is returned when there is no sock for a given ID.
var ErrNotFound = errors.New("not found")

// ErrDBConnection is returned when connection with the database fails.
var ErrDBConnection = errors.New("database connection error")

var baseQuery = "SELECT sock.sock_id AS id, sock.name, sock.description, sock.price, sock.count, sock.image_url_1, sock.image_url_2, array_to_string(array_agg(tag.name), ',') AS tag_name FROM sock JOIN sock_tag ON sock.sock_id=sock_tag.sock_id JOIN tag ON sock_tag.tag_id=tag.tag_id"

var DEFAULT_PAGE_NUM = 1
var DEFAULT_PAGE_SIZE = 100

// NewCatalogueService returns an implementation of the Service interface,
// with connection to an SQL database.
func NewCatalogueService(db *sqlx.DB, logger log.Logger) Service {
	return &catalogueService{
		db:     db,
		logger: logger,
	}
}

type catalogueService struct {
	db     *sqlx.DB
	logger log.Logger
}

func (s *catalogueService) List(tags []string, order string, pageNum, pageSize int) ([]Sock, error) {
	var socks []Sock
	query := baseQuery

	var args []interface{}

	for i, t := range tags {
		if i == 0 {
			query += " WHERE tag.name=$1"
			args = append(args, t)
		} else {
			query += fmt.Sprintf(" OR tag.name=$%d", i + 1)
			args = append(args, t)
		}
	}

	query += " GROUP BY id"

	if order != "" {
		query += fmt.Sprintf(" ORDER BY %s ASC", order)
	}

	query += pagination(pageNum, pageSize) + ";"

	err := s.db.Select(&socks, query, args...)
	if err != nil {
		s.logger.Log("database error", err)
		return []Sock{}, ErrDBConnection
	}
	for i, s := range socks {
		socks[i].ImageURL = []string{s.ImageURL_1, s.ImageURL_2}
		socks[i].Tags = strings.Split(s.TagString, ",")
	}

	return socks, nil
}

func (s *catalogueService) Count(tags []string) (int, error) {
	query := "SELECT COUNT(DISTINCT sock.sock_id) FROM sock JOIN sock_tag ON sock.sock_id=sock_tag.sock_id JOIN tag ON sock_tag.tag_id=tag.tag_id"

	var args []interface{}

	for i, t := range tags {
		if i == 0 {
			query += " WHERE tag.name=$1"
			args = append(args, t)
		} else {
			query += fmt.Sprintf(" OR tag.name=$%d", i + 1)
			args = append(args, t)
		}
	}

	query += ";"

	sel, err := s.db.Prepare(query)

	if err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}
	defer sel.Close()

	var count int
	err = sel.QueryRow(args...).Scan(&count)

	if err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}

	return count, nil
}

func (s *catalogueService) Get(id string) (Sock, error) {
	query := baseQuery + " WHERE sock.sock_id=$1 GROUP BY id;"

	var sock Sock
	err := s.db.Get(&sock, query, id)
	if err != nil {
		s.logger.Log("database error", err)
		return Sock{}, ErrNotFound
	}

	sock.ImageURL = []string{sock.ImageURL_1, sock.ImageURL_2}
	sock.Tags = strings.Split(sock.TagString, ",")

	return sock, nil
}

func (s *catalogueService) Health() []Health {
	var health []Health
	dbstatus := "OK"

	err := s.db.Ping()
	if err != nil {
		dbstatus = "err"
	}

	app := Health{"catalogue", "OK", time.Now().String()}
	db := Health{"catalogue-db", dbstatus, time.Now().String()}

	health = append(health, app)
	health = append(health, db)

	return health
}

func (s *catalogueService) Tags() ([]string, error) {
	var tags []string
	query := "SELECT name FROM tag;"
	rows, err := s.db.Query(query)
	if err != nil {
		s.logger.Log("database error", err)
		return []string{}, ErrDBConnection
	}
	var tag string
	for rows.Next() {
		err = rows.Scan(&tag)
		if err != nil {
			s.logger.Log("database error", err)
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func pagination(pageNum, pageSize int) string {
	if pageNum < 1 {
		pageNum = DEFAULT_PAGE_NUM
	}
	if pageSize < 1 {
		pageSize = DEFAULT_PAGE_SIZE
	}
	return fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, offset(pageNum, pageSize))
}

func offset(pageNum, pageSize int) int {
	return ((pageNum - 1) * pageSize)
}
