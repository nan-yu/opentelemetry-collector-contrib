// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statsreader

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type DatabaseReader struct {
	database *datasource.Database
	logger   *zap.Logger
	readers  []Reader
}

func NewDatabaseReader(ctx context.Context,
	parsedMetadata []*metadata.MetricsMetadata,
	databaseID *datasource.DatabaseID,
	serviceAccountPath string,
	readerConfig ReaderConfig,
	logger *zap.Logger) (*DatabaseReader, error) {

	database, err := datasource.NewDatabase(ctx, databaseID, serviceAccountPath)
	if err != nil {
		return nil, fmt.Errorf("error occurred during client instantiation for database %q: %w", databaseID.ID(), err)
	}

	readers := initializeReaders(logger, parsedMetadata, database, readerConfig)

	return &DatabaseReader{
		database: database,
		logger:   logger,
		readers:  readers,
	}, nil
}

func initializeReaders(logger *zap.Logger, parsedMetadata []*metadata.MetricsMetadata,
	database *datasource.Database, readerConfig ReaderConfig) []Reader {
	readers := make([]Reader, len(parsedMetadata))

	for i, mData := range parsedMetadata {
		switch mData.MetadataType() {
		case metadata.MetricsMetadataTypeCurrentStats:
			readers[i] = newCurrentStatsReader(logger, database, mData, readerConfig)
		case metadata.MetricsMetadataTypeIntervalStats:
			readers[i] = newIntervalStatsReader(logger, database, mData, readerConfig)
		}
	}

	return readers
}

func (databaseReader *DatabaseReader) Name() string {
	return databaseReader.database.DatabaseID().ID()
}

func (databaseReader *DatabaseReader) Shutdown() {
	databaseReader.logger.Debug("Closing connection to database",
		zap.String("database", databaseReader.database.DatabaseID().ID()))
	databaseReader.database.Client().Close()
}

func (databaseReader *DatabaseReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	databaseReader.logger.Debug("Executing read method for database",
		zap.String("database", databaseReader.database.DatabaseID().ID()))

	var result []*metadata.MetricsDataPoint

	for _, reader := range databaseReader.readers {
		dataPoints, err := reader.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot read data for data points databaseReader %q because of an error: %w",
				reader.Name(), err)
		}

		result = append(result, dataPoints...)
	}

	return result, nil
}
