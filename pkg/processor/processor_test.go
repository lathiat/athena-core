package processor

import (
	"context"
	"encoding/json"
	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/common/db"
	"github.com/canonical/athena-core/pkg/common/test"
	"github.com/canonical/athena-core/pkg/config"
	"github.com/go-orm/gorm"
	_ "github.com/go-orm/gorm/dialects/sqlite"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/memory"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"testing"
	"time"
)

type ProcessorTestSuite struct {
	suite.Suite
	config *config.Config
	db     *gorm.DB
}

func init() {
	logrus.SetOutput(ioutil.Discard)
}

func (s *ProcessorTestSuite) SetupTest() {
	s.config, _ = config.NewConfigFromBytes([]byte(test.DefaultTestConfig))
	s.db, _ = gorm.Open("sqlite3", "file::memory:?cache=shared")
	s.db.AutoMigrate(db.File{}, db.Report{})
}

type MockSubscriber struct {
	mock.Mock
	Options pubsub.HandlerOptions
}

func (s *MockSubscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

func (s *ProcessorTestSuite) TestRunProcessor() {
	filesComClient := test.FilesComClient{}
	salesforceClient := test.SalesforceClient{}

	provider := &memory.MemoryProvider{}
	processor, _ := NewProcessor(&filesComClient, &salesforceClient, provider, s.config, s.db)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	b, _ := json.Marshal(db.File{Path: "/uploads/sosreport-123.tar.xz"})
	b1, _ := json.Marshal(db.File{Path: "/uploads/sosreport-321.tar.xz"})
	b2, _ := json.Marshal(db.File{Path: "/uploads/sosreport-abc.tar.xz"})

	_ = provider.Publish(context.Background(), "sosreports", &pubsub.Msg{Data: b})
	_ = provider.Publish(context.Background(), "sosreports", &pubsub.Msg{Data: b1})
	_ = provider.Publish(context.Background(), "sosreports", &pubsub.Msg{Data: b2})

	var called = 0

	_ = processor.Run(ctx, func(fc common.FilesComClient, sf common.SalesforceClient,
		name string, topic string, reports map[string]config.Report, cfg *config.Config, dbConn *gorm.DB) pubsub.Subscriber {
		var subscriber = MockSubscriber{Options: pubsub.HandlerOptions{
			Topic:   topic,
			Name:    "athena-processor-" + name,
			AutoAck: false,
			JSON:    true,
		}}

		subscriber.Options.Handler = func(ctx context.Context, msg *db.File, m *pubsub.Msg) error {
			called++
			return nil
		}
		return &subscriber
	})

	assert.Equal(s.T(), called, 3)
	assert.Equal(s.T(), len(provider.Msgs["sosreports"]), 3)
}

func TestNewProcessor(t *testing.T) {
	suite.Run(t, &ProcessorTestSuite{})
}
