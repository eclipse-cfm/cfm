//  Copyright (c) 2025 Metaform Systems, Inc
//
//  This program and the accompanying materials are made available under the
//  terms of the Apache License, Version 2.0 which is available at
//  https://www.apache.org/licenses/LICENSE-2.0
//
//  SPDX-License-Identifier: Apache-2.0
//
//  Contributors:
//       Metaform Systems, Inc. - initial API and implementation
//

package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
)

// DatabaseRecord represents a complete database row with all column values
type DatabaseRecord struct {
	// Values contains all column values (key: column name, value: column value)
	// The conversion functions determine how to interpret and serialize/deserialize each value
	Values map[string]any
}

// NewPostgresEntityStore creates a new PostgresEntityStore instance
func NewPostgresEntityStore[T store.EntityType](
	tableName string,
	columnNames []string,
	recordToEntity func(*sql.Tx, *DatabaseRecord) (T, error),
	entityToRecord func(T) (*DatabaseRecord, error),
	builder JSONBSQLBuilder) *PostgresEntityStore[T] {

	return &PostgresEntityStore[T]{
		tableName:      tableName,
		columnNames:    columnNames,
		recordToEntity: recordToEntity,
		entityToRecord: entityToRecord,
		builder:        builder,
		matcher:        &query.DefaultFieldMatcher{},
	}
}

// PostgresEntityStore implements store.EntityStore for Postgres databases
// The store must be customized for the entity type it manages, including table name, columns, record transformers, and
// the JSONB SQL builder.
type PostgresEntityStore[T store.EntityType] struct {
	tableName      string
	columnNames    []string
	matcher        query.FieldMatcher
	recordToEntity func(*sql.Tx, *DatabaseRecord) (T, error)
	entityToRecord func(T) (*DatabaseRecord, error)
	builder        JSONBSQLBuilder
}

func (p *PostgresEntityStore[T]) FindByID(ctx context.Context, id string) (T, error) {
	selectClause := strings.Join(p.columnNames, ", ")

	tx := getTxFromContext(ctx)
	row := tx.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", selectClause, p.tableName),
		id,
	)

	// Create slices to hold scan values
	scanValues := make([]any, len(p.columnNames))
	for i := range scanValues {
		scanValues[i] = new(any)
	}

	err := row.Scan(scanValues...)
	if err != nil {
		var zero T
		if errors.Is(err, sql.ErrNoRows) {
			return zero, types.ErrNotFound
		}
		return zero, fmt.Errorf("failed to query entity: %w", err)
	}

	record := p.buildRecordFromScan(scanValues)
	return p.recordToEntity(tx, &record)
}

func (p *PostgresEntityStore[T]) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	row := getTxFromContext(ctx).QueryRowContext(ctx,
		fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)", p.tableName),
		id,
	)
	err := row.Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check entity existence: %w", err)
	}
	return exists, nil
}

func (p *PostgresEntityStore[T]) Create(ctx context.Context, entity T) (T, error) {
	record, err := p.entityToRecord(entity)
	if err != nil {
		return entity, fmt.Errorf("failed to convert entity to record: %w", err)
	}

	columnNames := make([]string, 0, len(record.Values))
	values := make([]any, 0, len(record.Values))
	placeholders := make([]string, 0, len(record.Values))

	paramIndex := 1

	// Add all columns in order they appear in p.columnNames
	for _, colName := range p.columnNames {
		if val, exists := record.Values[colName]; exists {
			columnNames = append(columnNames, colName)
			values = append(values, val)
			placeholders = append(placeholders, fmt.Sprintf("$%d", paramIndex))
			paramIndex++
		}
	}

	selectClause := strings.Join(p.columnNames, ", ")

	// Create scan destinations for all returning columns
	scanValues := make([]any, len(p.columnNames))
	for i := range scanValues {
		scanValues[i] = new(any)
	}

	err = getTxFromContext(ctx).QueryRowContext(ctx,
		fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
			p.tableName, strings.Join(columnNames, ", "),
			strings.Join(placeholders, ", "), selectClause),
		values...,
	).Scan(scanValues...)

	if err != nil {
		return entity, fmt.Errorf("failed to create entity: %w", err)
	}

	// Convert scan results to entity using the conversion function
	returnedRecord := p.buildRecordFromScan(scanValues)
	return p.recordToEntity(getTxFromContext(ctx), &returnedRecord)
}

func (p *PostgresEntityStore[T]) Update(ctx context.Context, entity T) error {
	record, err := p.entityToRecord(entity)
	if err != nil {
		return fmt.Errorf("failed to convert entity to record: %w", err)
	}

	setClauses := make([]string, 0)
	values := make([]any, 0)
	paramIndex := 1

	// Build SET clauses for all columns (skip id)
	for _, colName := range p.columnNames {
		if colName == "id" {
			continue
		}
		if val, exists := record.Values[colName]; exists {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", colName, paramIndex))
			values = append(values, val)
			paramIndex++
		}
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no columns to update")
	}

	// Add id to WHERE clause
	values = append(values, entity.GetID())
	idPlaceholder := fmt.Sprintf("$%d", paramIndex)

	result, err := getTxFromContext(ctx).ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET %s WHERE id = %s",
			p.tableName, strings.Join(setClauses, ", "), idPlaceholder),
		values...,
	)

	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return types.ErrNotFound
	}

	return nil
}

func (p *PostgresEntityStore[T]) Delete(ctx context.Context, id string) error {
	result, err := getTxFromContext(ctx).ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE id = $1", p.tableName),
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return types.ErrNotFound
	}

	return nil
}

func (p *PostgresEntityStore[T]) GetAll(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		selectClause := strings.Join(p.columnNames, ", ")

		tx := getTxFromContext(ctx)
		rows, err := tx.QueryContext(ctx,
			fmt.Sprintf("SELECT %s FROM %s", selectClause, p.tableName),
		)
		if err != nil {
			yield(*(new(T)), fmt.Errorf("failed to query entities: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			scanValues := make([]any, len(p.columnNames))
			for i := range scanValues {
				scanValues[i] = new(any)
			}

			err := rows.Scan(scanValues...)
			if err != nil {
				if !yield(*(new(T)), fmt.Errorf("failed to scan entity: %w", err)) {
					return
				}
				continue
			}

			record := p.buildRecordFromScan(scanValues)

			entity, err := p.recordToEntity(tx, &record)
			if err != nil {
				if !yield(*(new(T)), fmt.Errorf("failed to convert record to entity: %w", err)) {
					return
				}
				continue
			}

			if !yield(entity, nil) {
				return
			}
		}

		if err := rows.Err(); err != nil {
			yield(*(new(T)), fmt.Errorf("iteration error: %w", err))
		}
	}
}

func (p *PostgresEntityStore[T]) GetAllCount(ctx context.Context) (int64, error) {
	var count int64
	row := getTxFromContext(ctx).QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", p.tableName),
	)
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count entities: %w", err)
	}
	return count, nil
}

func (p *PostgresEntityStore[T]) GetAllPaginated(ctx context.Context, opts store.PaginationOptions) iter.Seq2[T, error] {
	return p.queryEntities(ctx, nil, &opts)
}

func (p *PostgresEntityStore[T]) FindByPredicate(ctx context.Context, predicate query.Predicate) iter.Seq2[T, error] {
	return p.queryEntities(ctx, predicate, nil)
}

func (p *PostgresEntityStore[T]) FindByPredicatePaginated(ctx context.Context, predicate query.Predicate, opts store.PaginationOptions) iter.Seq2[T, error] {
	return p.queryEntities(ctx, predicate, &opts)
}

func (p *PostgresEntityStore[T]) FindFirstByPredicate(ctx context.Context, predicate query.Predicate) (T, error) {
	var result T
	var lastErr error
	found := false

	// Use pagination options with Limit=1 to get only the first result
	opts := store.PaginationOptions{Limit: 1}
	for entity, err := range p.queryEntities(ctx, predicate, &opts) {
		if err != nil {
			lastErr = err
			continue
		}
		result = entity
		found = true
		break
	}

	if !found {
		if lastErr != nil {
			return result, lastErr
		}
		return result, types.ErrNotFound
	}

	return result, nil
}

func (p *PostgresEntityStore[T]) CountByPredicate(ctx context.Context, predicate query.Predicate) (int64, error) {
	var count int64
	var whereClause string
	var args []any

	tx := getTxFromContext(ctx)

	// Build WHERE clause if predicate is provided
	if predicate != nil {
		whereClause, args = p.builder.BuildSQL(predicate)
	}

	// Build the count query
	var queryStr string
	if whereClause != "" {
		queryStr = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", p.tableName, whereClause)
	} else {
		queryStr = fmt.Sprintf("SELECT COUNT(*) FROM %s", p.tableName)
	}

	row := tx.QueryRowContext(ctx, queryStr, args...)
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count entities: %w", err)
	}
	return count, nil
}

func (p *PostgresEntityStore[T]) DeleteByPredicate(ctx context.Context, predicate query.Predicate) error {
	var whereClause string
	var args []any

	tx := getTxFromContext(ctx)

	// Build WHERE clause if predicate is provided
	if predicate != nil {
		whereClause, args = p.builder.BuildSQL(predicate)
	}

	// Build the delete query
	var queryStr string
	if whereClause != "" {
		queryStr = fmt.Sprintf("DELETE FROM %s WHERE %s", p.tableName, whereClause)
	} else {
		queryStr = fmt.Sprintf("DELETE FROM %s", p.tableName)
	}

	result, err := tx.ExecContext(ctx, queryStr, args...)
	if err != nil {
		return fmt.Errorf("failed to delete entities: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return types.ErrNotFound
	}

	return nil
}

// queryEntities queries entities with optional filtering and pagination
func (p *PostgresEntityStore[T]) queryEntities(
	ctx context.Context,
	predicate query.Predicate,
	opts *store.PaginationOptions,
) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		selectClause := strings.Join(p.columnNames, ", ")
		var whereClause string
		var args []any

		tx := getTxFromContext(ctx)

		// Build WHERE clause if predicate is provided
		if predicate != nil {
			whereClause, args = p.builder.BuildSQL(predicate)
		}

		// Build the query
		var queryStr string
		if whereClause != "" {
			queryStr = fmt.Sprintf("SELECT %s FROM %s WHERE %s", selectClause, p.tableName, whereClause)
		} else {
			queryStr = fmt.Sprintf("SELECT %s FROM %s", selectClause, p.tableName)
		}

		// Add pagination if provided
		if opts != nil && opts.Limit > 0 {
			queryStr = fmt.Sprintf("%s ORDER BY id LIMIT $%d OFFSET $%d", queryStr, len(args)+1, len(args)+2)
			args = append(args, opts.Limit, opts.Offset)
		} else if opts != nil && opts.Offset > 0 {
			// Handle offset without limit
			queryStr = fmt.Sprintf("%s ORDER BY id OFFSET $%d", queryStr, len(args)+1)
			args = append(args, opts.Offset)
		}

		rows, err := tx.QueryContext(ctx, queryStr, args...)
		if err != nil {
			yield(*(new(T)), fmt.Errorf("failed to query entities: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			scanValues := make([]any, len(p.columnNames))
			for i := range scanValues {
				scanValues[i] = new(any)
			}

			err := rows.Scan(scanValues...)
			if err != nil {
				if !yield(*(new(T)), fmt.Errorf("failed to scan entity: %w", err)) {
					return
				}
				continue
			}

			record := p.buildRecordFromScan(scanValues)

			entity, err := p.recordToEntity(tx, &record)
			if err != nil {
				if !yield(*(new(T)), fmt.Errorf("failed to convert record to entity: %w", err)) {
					return
				}
				continue
			}

			if !yield(entity, nil) {
				return
			}
		}

		if err := rows.Err(); err != nil {
			yield(*(new(T)), fmt.Errorf("iteration error: %w", err))
		}
	}
}

func (p *PostgresEntityStore[T]) buildRecordFromScan(scanValues []any) DatabaseRecord {
	record := DatabaseRecord{
		Values: make(map[string]any),
	}

	// Store all column data - conversion functions know what to do with each value
	for i, colName := range p.columnNames {
		val := *scanValues[i].(*any)
		record.Values[colName] = val
	}

	return record
}

func getTxFromContext(ctx context.Context) *sql.Tx {
	return ctx.Value(SQLTransactionKey).(*sql.Tx)
}
