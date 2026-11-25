# The CFM Query Language

## Overview

The CFM query language is a predicate-based system designed to work with both in-memory storage and Postgres
databases. It uses [ANTLR](https://www.antlr.org/) for parsing human-readable query syntax into
structured predicate objects that can be evaluated against any data source.

## Architecture

### Core Components

1. **Query String Syntax** - Human-readable query expressions (parsed via ANTLR grammar)
2. **Predicates** - Abstract representation of query conditions
3. **Field Matcher** - Strategy pattern for field value extraction and comparison
4. **Storage Adapters** - Implementations for in-memory and database backends

## Query Syntax

The query language supports both simple and complex query expressions.

### Basic Syntax Structure

`field operator value`

### Comparison Operators

| Operator | Meaning               | Example              |
|----------|-----------------------|----------------------|
| =        | Equal                 | status = 'active'    |
| !=       | Not equal             | status != 'inactive' |
| \>       | Greater than          | count > 10           |
| \>=      | Greater than or equal | count >= 10          |
| <        | Less than             | count < 100          |
| <=       | Less than or equal    | count <= 100         |

### String Operations

| Operator    | Meaning                        | Example                       |
|-------------|--------------------------------|-------------------------------|
| LIKE        | Pattern matching (database)    | name LIKE 'John%'             |
| CONTAINS    | Substring contains (in-memory) | description CONTAINS 'urgent' |
| STARTS_WITH | Prefix matching                | code STARTS_WITH 'ERR'        |
| ENDS_WITH   | Suffix matching                | filename ENDS_WITH '.log'     |

## Collection Operations

| Operator | Meaning          | Example                               |
|----------|------------------|---------------------------------------|
| IN       | Value in set     | status IN ('active', 'pending')       |
| NOT IN   | Value not in set | status NOT IN ('deleted', 'archived') |

## NULL Checks

| Operator    | Meaning           | Example                 |
|-------------|-------------------|-------------------------|
| IS NULL     | Field is null     | description IS NULL     |
| IS NOT NULL | Field is not null | description IS NOT NULL |

### Compound Predicates

Combine multiple conditions using logical operators:

**AND Operator** - All conditions must be true:

`field1 = 'value1' AND field2 > 100 AND field3 STARTS_WITH 'prefix'`

**OR Operator** - At least one condition must be true:

`status = 'active' OR status = 'pending' OR status = 'processing'`

**Nested Expressions** with parentheses:

`(status = 'active' OR status = 'pending') AND priority > 5`

### Example Queries

Simple equality:

`correlationId = '12345'`

Nested object field access:

`properties.group = 'suppliers'`

Complex filtering:

`(state = 'RUNNING' AND priority >= 1) OR (state = 'PENDING' AND createdTimestamp > '2025-01-01')`

String operations:

`orchestrationType STARTS_WITH 'provisioning' AND state != 'FAILED'`

## Predicate System

### Predicate Interface

All predicates implement the Predicate interface with two methods:

``` go
type Predicate interface {
    // Matches evaluates the predicate against an object
    Matches(obj any, matcher FieldMatcher) bool
    
    // String returns a human-readable representation
    String() string
}
```

### Atomic Predicates

Single condition predicates consisting of:

- **Field** - The object property to test
- **Operator** - The comparison operation
- **Value** - The comparison value

``` go
predicate := &AtomicPredicate{
    Field:    "state",
    Operator: "=",
    Value:    "RUNNING",
}
```

### Compound Predicates

Logical combinations of multiple predicates:

``` go
query.And(
    query.Or(
        query.Eq("state", "RUNNING"),
        query.Eq("state", "PENDING"),
    ),
    query.Gte("priority", 1),
)
```

## Field Matcher Strategy

The `FieldMatcher` interface enables custom logic for different data types.

### Default Field Matcher

The `DefaultFieldMatcher` provides reflection-based matching: Navigates nested object structures using dot notation (
e.g.,
properties.group) Supports standard Go types (strings, numbers, times, booleans) Type-safe comparisons

### Custom Field Matchers

Different storage backends can implement custom matchers for optimization: In-Memory: Direct object navigation
Postgres: Field extraction from JSON/JSONB columns Other DBs: Adapter-specific implementations.

## In-Memory Storage Implementation

### Query Processing Flow

``` 
1. Query String (e.g., "status = 'active' AND count > 10")
↓
2. ANTLR Parser → Predicate Object
↓
3. InMemoryEntityStore.FindByPredicate()
↓
4. Iterate entities and evaluate Predicate.Matches()
↓
5. Return matching entities
```

## Postgres Storage Implementation

### Query Translation

```
Predicate Object → SQL WHERE Clause → Postgres Query
```

### Operator Mapping

| Predicate Operator | SQL Translation                                     |
|--------------------|-----------------------------------------------------|
| =                  | column = value                                      |
| !=                 | column != value                                     |
| \>                 | column > value                                      |
| IN                 | column IN (...)                                     |
| LIKE               | column LIKE pattern                                 |
| CONTAINS           | column::text @> value (JSONB) or substring matching |
| IS NULL            | column IS NULL                                      |

### Example Translation

Predicate:

``` go
query.And(
    query.Eq("state", "active"),
    query.Gt("priority", 5),
)
```

SQL WHERE Clause:

`WHERE state = 'active' AND priority > 5`

## CFM Entities

The query language can be used to query CFM entities:

- **Orchestrations**: Filter by state, type, correlation ID, timestamps, for example with
  `POST /api/v1alpha1/orchestrations/query` and the body `{"predicate": "state = 3", "offset": 0, "limit": 50}`
- **Definitions**: Search orchestration and activity definitions
- **Entities of a manager**: A manager that builds on the engine can use the query language for its own entities. The
  Tenant Manager of [cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace) uses it for tenants, participant
  profiles and VPA metadata, which the examples with `vpas` below are taken from

### JSONB Support

JSONB query types enable querying of JSON-structured data stored in Postgres JSONB columns. The system supports two
primary JSONB field type categories with different query semantics. 

### JSONB Field Types 

The `JSONBFieldType` enum defines how JSONB fields should be handled during query construction:

#### JSONBFieldTypeScalar

- **Purpose**: Direct accessor for scalar JSONB values or nested objects
- **Use Case**: Simple properties, nested configuration objects, metadata fields
- **SQL Translation**: Uses direct path accessors (`->>` for text, `->` for JSON)
- **Example**: `metadata.environment`, `config.database.host`
- **Query Pattern**: Direct comparison without array traversal

``` go
builder.WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
   "metadata": sqlstore.JSONBFieldTypeScalar,
})
```

#### JSONBFieldTypeArrayOfObjects

- **Purpose**: Query arrays containing object elements
- **Use Case**: Arrays of complex entities like VPAs, activities, profiles
- **SQL Translation**: Uses `jsonb_array_elements()` for element iteration
- **Example**: `vpas.type`, `vpas.cell.id`, `activities.name`
- **Query Pattern**: Searches across array elements using EXISTS conditions

``` go
builder.WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
   "vpas": sqlstore.JSONBFieldTypeArrayOfObjects,
   "activities": sqlstore.JSONBFieldTypeArrayOfObjects,
})

```

#### JSONBFieldTypeArrayOfScalars

- **Purpose**: Query arrays containing scalar values (strings, numbers)
- **Use Case**: Tag lists, state arrays, simple value collections
- **SQL Translation**: Uses `jsonb_array_elements()` with scalar comparisons
- **Example**: Array of strings or numbers

``` go
builder.WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
   "tags": sqlstore.JSONBFieldTypeArrayOfScalars,
})

```

#### JSONBFieldTypeMapOfArrays

- **Purpose**: Query nested maps containing array values
- **Use Case**: Dynamic configurations, indexed collections, multi-valued properties
- **SQL Translation**: Uses `jsonb_each()` for map iteration combined with array element access
- **Example**: `config.environments`, `settings.regions`, `features.flags`
- **Query Pattern**: Searches across map entries and their array elements using EXISTS conditions


### Configuration

#### Setting Up JSONB Fields

Create a and configure field types: `PostgresJSONBBuilder

``` go 
builder := sqlstore.NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
   "metadata":    sqlstore.JSONBFieldTypeScalar,
   "vpas":        sqlstore.JSONBFieldTypeArrayOfObjects,
   "activities":  sqlstore.JSONBFieldTypeArrayOfObjects,
   "tags":        sqlstore.JSONBFieldTypeArrayOfScalars,
})
```

Or using `WithJSONBFields()` with default (ArrayOfObjects):

``` go
builder := sqlstore.NewPostgresJSONBBuilder().WithJSONBFields("vpas", "activities")
```

### Supported Query Operators

#### Comparison Operators

| Operator            | JSONB SQL Pattern                 | Field Type Support | Use Case                    |
|---------------------|-----------------------------------|--------------------|-----------------------------|
| `=` (Equal)         | Direct comparison or array EXISTS | All types          | Exact value match           |
| `!=` (NotEqual)     | Direct comparison or array EXISTS | All types          | Exclude specific values     |
| `>` (Greater)       | Numeric cast comparison           | Scalar, Array      | Numeric thresholds          |
| `<` (Less)          | Numeric cast comparison           | Scalar, Array      | Numeric thresholds          |
| `>=` (GreaterEqual) | Numeric cast comparison           | Scalar, Array      | Numeric ranges              |
| `<=` (LessEqual)    | Numeric cast comparison           | Scalar, Array      | Numeric ranges              |
| `IN`                | Array of values                   | All types          | Multiple value matching     |
| `NOT IN`            | Array of values                   | All types          | Exclude multiple values     |
| `IS NULL`           | NULL check                        | All types          | Missing field detection     |
| `IS NOT NULL`       | NOT NULL check                    | All types          | Field presence check        |
| `@>` (Contains)     | JSONB contains operator           | Scalar             | Document/object containment |

### Query Examples

#### Scalar JSONB Field Queries

Query nested object properties:

``` go
// metadata.environment = 'prod'
predicate := query.Eq("metadata.environment", "prod")

// metadata.config.database.host = 'localhost'
predicate := query.Eq("metadata.config.database.host", "localhost")

// metadata.score > 75
predicate := query.Gt("metadata.score", 75)
```

#### Array of Objects Queries

Query properties of array elements:

``` go
// VPA array: vpas.type = 'connector'
predicate := query.Eq("vpas.type", "connector")

// Nested object in array: vpas.cell.id = 'cell-prod'
predicate := query.Eq("vpas.cell.id", "cell-prod")

// Multiple conditions on array: vpas.type = 'connector' AND vpas.state = 'active'
predicate := query.And(
    query.Eq("vpas.type", "connector"),
    query.Eq("vpas.state", "active"),
)
```

#### Numeric Comparisons

``` go
// metadata.priority > 5
predicate := query.Gt("metadata.priority", 5)

// metadata.priority <= 10
predicate := query.Lte("metadata.priority", 10)

// metadata.score IN (70, 80, 90)
predicate := query.In("metadata.score", 70, 80, 90)
```

#### NULL Checks

``` go
// metadata.status IS NULL
predicate := query.IsNull("metadata.status")

// vpas.cell IS NOT NULL
predicate := query.IsNotNull("vpas.cell")
```

#### Complex Compound Queries

``` go
// (type = "connector" AND state = "active") OR (type = "credential-service")
predicate := query.Or(
    query.And(
        query.Eq("vpas.type", "connector"),
        query.Eq("vpas.state", "active"),
    ),
    query.Eq("vpas.type", "credential-service"),
)

// metadata.environment = "prod" AND metadata.region = "us-east"
predicate := query.And(
    query.Eq("metadata.environment", "prod"),
    query.Eq("metadata.region", "us-east"),
)

// metadata.status IN ("active", "pending")
predicate := query.In("metadata.status", "active", "pending")
```

#### Real-World Usage Examples

##### Tenant Manager (`tmanager/sqlstore` in [cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace))

Querying participant profiles with VPAs:

- Filter by VPA type: `vpas.type = 'connector'`
- Filter by VPA cell: `vpas.cell.id = 'prod-cell-1'`
- Complex: `vpas.type = 'connector' AND vpas.state != 'disposed'`

Querying tenant properties:

- Metadata matching: `properties.region = 'us-east'`
- Nested config: `properties.config.environment = 'prod'`

##### Provision Manager (`pmanager/sqlstore`)

Querying orchestration definitions:

- Array of objects: `activities.type = 'ProcessTask'`
- Nested array search: `schema.properties.name = 'processId'`
- Multiple activities: `activities.type IN ('ProcessTask', 'ServiceTask')`
