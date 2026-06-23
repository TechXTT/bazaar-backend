package algolia

import (
	"log"
	"time"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	algoliasearch "github.com/algolia/algoliasearch-client-go/v4/algolia/search"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

// ProductRecord is the data indexed per product.
type ProductRecord struct {
	ObjectID    string
	Name        string
	Description string
	Price       float64
	Unit        string
	ImageURL    string
	StoreID     string
	StoreName   string
	CreatedAt   time.Time
}

type (
	AlgoliaService interface {
		IndexProduct(p ProductRecord) error
		DeleteProduct(id string) error
		BulkIndex(products []ProductRecord) error

		// EnqueueIndex / EnqueueDelete submit an index mutation to a bounded,
		// retrying background queue (BE-19). They never block the caller and never
		// spawn an unbounded goroutine.
		EnqueueIndex(p ProductRecord)
		EnqueueDelete(id string)
	}

	algoliaService struct {
		client *algoliasearch.APIClient
		index  string
		queue  *queue
	}

	noopService struct{}
)

func init() {
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewAlgoliaService)
	})
}

func NewAlgoliaService(i *do.Injector) (AlgoliaService, error) {
	cfg := do.MustInvoke[config.Config](i).GetAlgolia()

	if cfg.AppID == "" {
		log.Println("algolia: ALGOLIA_APP_ID not set, using no-op service")
		return &noopService{}, nil
	}

	client, err := algoliasearch.NewClient(cfg.AppID, cfg.WriteKey)
	if err != nil {
		return nil, err
	}

	svc := &algoliaService{client: client, index: cfg.ProductsIndex}
	svc.queue = newQueue(svc)
	return svc, nil
}

func toMap(p ProductRecord) map[string]any {
	return map[string]any{
		"objectID":    p.ObjectID,
		"Name":        p.Name,
		"Description": p.Description,
		"Price":       p.Price,
		"Unit":        p.Unit,
		"ImageURL":    p.ImageURL,
		"StoreID":     p.StoreID,
		"StoreName":   p.StoreName,
		"CreatedAt":   p.CreatedAt.Unix(),
	}
}

func (s *algoliaService) IndexProduct(p ProductRecord) error {
	_, err := s.client.SaveObject(s.client.NewApiSaveObjectRequest(s.index, toMap(p)))
	if err != nil {
		log.Printf("algolia: IndexProduct %s: %v", p.ObjectID, err)
	}
	return err
}

func (s *algoliaService) DeleteProduct(id string) error {
	_, err := s.client.DeleteObject(s.client.NewApiDeleteObjectRequest(s.index, id))
	if err != nil {
		log.Printf("algolia: DeleteProduct %s: %v", id, err)
	}
	return err
}

func (s *algoliaService) BulkIndex(products []ProductRecord) error {
	if len(products) == 0 {
		return nil
	}
	objects := make([]map[string]any, len(products))
	for i, p := range products {
		objects[i] = toMap(p)
	}
	_, err := s.client.ChunkedBatch(s.index, objects, algoliasearch.ACTION_UPDATE_OBJECT)
	if err != nil {
		log.Printf("algolia: BulkIndex %d products: %v", len(products), err)
	}
	return err
}

func (s *algoliaService) EnqueueIndex(p ProductRecord) {
	s.queue.enqueue(indexJob{kind: jobIndex, record: p})
}

func (s *algoliaService) EnqueueDelete(id string) {
	s.queue.enqueue(indexJob{kind: jobDelete, id: id})
}

func (n *noopService) IndexProduct(_ ProductRecord) error { return nil }
func (n *noopService) DeleteProduct(_ string) error       { return nil }
func (n *noopService) BulkIndex(_ []ProductRecord) error  { return nil }
func (n *noopService) EnqueueIndex(_ ProductRecord)       {}
func (n *noopService) EnqueueDelete(_ string)             {}
