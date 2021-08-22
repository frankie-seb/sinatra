![sinatra logo](https://www.crimemuseum.org/wp-content/uploads/2015/04/sinatramug1.jpg)

Sinatra is a tool to generate a Go ORM tailored to your database schema.

It is a "database-first" ORM as opposed to "code-first" (like gorm/gorp).
That means you must first create your database schema. Please use something
like [sql-migrate](https://github.com/rubenv/sql-migrate)
or some other migration tool to manage this part of the database's life-cycle.

Table of Contents
=================

  * [Sinatra](#sinatra)
    * [Why another ORM](#why-another-orm)
    * [About Sinatra](#about-sinatra)
      * [Features](#features)
      * [Missing Features](#missing-features)
    * [Requirements](#requirements)
    * [How it works](#how-it-works)
    * [Getting started](#getting-started)
      * [Installation](#installation)
      * [Initial Generation](#initial-generation)
      * [Configuration](#configuration)
      * [Regeneration](#regeneration)
    * [Features &amp; Examples](#features--examples)
      * [General Generation](#complete-generation)
      * [Custom Queries/Mutations](#custom-queries-mutations)
      * [Federation](#federation)
        * [Extending Queries](#extending-queries)
        * [Dataloaders](#dataloaders)

## Why another ORM

While building a suite of federated services, generating the CRUD activities each time appeared to be overly tedious. Leveraging a database-first approach simplified quickly spinning up services and accessing the core data.


## About Sinatra

### Features

- Full model generation
- Extremely fast code generation
- High performance through generation & intelligent caching
- Uses boil.Executor (simple interface, sql.DB, sqlx.DB etc. compatible)
- Uses context.Context
- Easy workflow (models can always be regenerated, full auto-complete)
- Strongly typed querying (usually no converting or binding to pointers)
- Hooks (Before/After Create/Select/Update/Delete/Upsert)
- Automatic CreatedAt/UpdatedAt
- Automatic DeletedAt
- Relationships/Associations
- Eager loading (recursive)
- Custom struct tags
- Transactions
- Raw SQL fallback
- Compatibility tests (Run against your own DB schema)
- Debug logging
- Leverages sqlboiler and gqlgen under the hood

### Missing features

- Authorization
- ...plenty

## Requirements

* Go 1.13, older Go versions are not supported.
* Table names and column names should use `snake_case` format.
  * We require `snake_case` table names and column names. This is a recommended default in Postgres,
  and we agree that it's good form, so we're enforcing this format for all drivers for the time being.
* Join tables should use a *composite primary key*.
  * For join tables to be used transparently for relationships your join table must have
  a *composite primary key* that encompasses both foreign table foreign keys and
  no other columns in the table. For example, on a join table named
  `user_videos` you should have: `primary key(user_id, video_id)`, with both
  `user_id` and `video_id` being foreign key columns to the users and videos
  tables respectively and there are no other columns on this table.

## How it works

1. Structs and db functions are generated off the database schema via [sqlboiler](https://github.com/volatiletech/sqlboiler).
2. The GraphQL schema is created off these models in the `schema` folder.
3. The types and helpers to support accepting GraphQL queries are then generated and stored in the `graph` folder via [gqlgen](https://github.com/99designs/gqlgen).
4. The necessary CRUD resolvers are generated for each schema in the `resolvers` folder.
5. The helpers are generated to assist the resolvers in fetching and manipulating the data into the required relay format. These are stored in the `helpers` folder.
6. Where required, the relevant federation types and code is stored in the `graph` folder.

## Getting started

### Installation

To install sinatra run the command `go get github.com/FrankieHealth/sinatra` in your project directory.

### Initial Generation

You could initialize a new project using the recommended folder structure by running this command `go run github.com/FrankieHealth/sinatra init`.

### Configuration

Sinatra can be configured using a `sinatra.yml` file, by default it will be loaded from the current directory, or any parent directory.

Example:

```yml
# Where should the database models go?
model:
  dirname: models
  package: models
# Where should the generated helpers go?
helper:
  dirname: helpers
  package: helpers
# Where should the graphql models go?
graph:
  dirname: graph
  package: graph
# Where should the generated schema go?
schema:
  dirname: schema
  package: schema
# Where should the generated resolvers go?
resolver:
  dirname: resolvers
  package: resolvers
  type: separated
# Uncomment to enable federation
federation:
  activate: true
# What's the db config?
database:
  dbname: main
  # user where federation is active
  schema: public
  host: localhost
  port: 5432
  user: admin
  password: 1234
  sslmode: disable
  blacklist: ["gorp_migrations", "migrations", "knex_migrations_new", "knex_migrations_new_lock"]
  whitelist: ["users"]
```

### Regeneration

To regenerate and check the integrity of your project run `sinatra` in your project directory.

## Features &amp; Examples

### General Generation

When a new change has been made to your database, run `sinatra` in your project directory to update the API, relative helpers and resolvers.

To overwrite a particular function in a resolver, simply create a file with the same name omitting the `_gen` suffix, e.g. `user.go` for `user_gen.go`. Create the new function in this file and run `sinatra`, the original function will be commented out.

### Custom Queries/Mutations

Adding a new query is as simple as creating a new file in the `schema` folder without the suffix `_gen`, e.g. `user.go` for `user_gen.go`. Extend either the query or mutation type, create the query/mutation and required types.

```
extend type Mutation {
  updateUserProfilePassword(id: ID!, input: UpdateUserProfilePassword!): Boolean!
}

input UpdateUserProfilePassword {
  oldPassword: String!
  newPassword: String!
}
```

### Federation

#### Extending Queries 

Extending a query for a federated server is a four step process.

1. Extend the type where the data for that field is to be resolved.

```
extend type Event @key(fields: "id") {
  id: ID! @external
  userId: ID! @external
  createdBy: ID @external
  userProfile: UserProfile! @requires(fields: "userId")
  createdByUserProfile: UserProfile @requires(fields: "createdBy")
  createdByUserProfileAccount: Account @requires(fields: "createdBy")
}
```

2. Create the file `federation_models.go` in `graph` and define the extended type external fields.

```
type Event struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`
	CreatedBy *string `json:"createdBy"`
}
```

3. In the relevant resolver, e.g. `event.go`, add the required resolver, findBys and resolver functions.

```
type eventResolver struct{ *Resolver }

func (r *Resolver) Event() fm.EventResolver {
	return &eventResolver{r}
}

func (r *entityResolver) FindEventByID(ctx context.Context, id string) (*fm.Event, error) {
	return &fm.Event{
		ID: id,
	}, nil
}

func (r *eventResolver) UserProfile(ctx context.Context, obj *fm.Event) (*fm.UserProfile, error) {
	id := base_helpers.IDToBoilerInt(obj.UserID)
	return dataloader.CtxLoaders(ctx).UserProfile.Load(id)
}

func (r *eventResolver) CreatedByUserProfile(ctx context.Context, obj *fm.Event) (*fm.UserProfile, error) {
	if obj.CreatedBy == nil {
		return nil, nil
	}
	id := base_helpers.IDToBoilerInt(*obj.CreatedBy)
	return dataloader.CtxLoaders(ctx).UserProfile.Load(id)
}

func (r *eventResolver) CreatedByUserProfileAccount(ctx context.Context, obj *fm.Event) (*fm.Account, error) {
	if obj.CreatedBy == nil {
		return nil, nil
	}
	id := bh.IDToBoilerInt(*obj.CreatedBy)
	return dataloader.CtxLoaders(ctx).AccountByUser.Load(id)
}
```

4. Create the dataloader if required (see below).

#### Dataloaders 

Dataloaders assist in avoiding the N+1 issues that are commonplace with GraphQL queries.

1. As per `https://github.com/vektah/dataloaden`, generate a new dataloader, e.g. `go run github.com/vektah/dataloaden UserLoader string *github.com/be-auth/graph.User`
2. Create the corresponding dataloader file `dataloader/userloader.go`
3. Add the dataloader to `dataloader/dataloader.go`.

It's possible to create dataloaders that return an array vs single.