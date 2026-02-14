# Recognized Domains

agent-evals uses keyword matching and explicit `domains` fields to classify what each agent covers. This file is the canonical reference for all recognized domains, their keywords, and the boundary probes used during live testing.

## Software Engineering

### backend

Keywords: backend, server, api, rest, graphql, grpc, microservice, service layer, business logic, middleware, endpoint, request handling, http server

Boundary probes test against: frontend, devops, databases

### frontend

Keywords: frontend, front-end, react, vue, angular, svelte, css, html, browser, dom, ui component, web app, responsive, accessibility, a11y, tailwind, next.js, nuxt

Boundary probes test against: backend, devops

### databases

Keywords: database, sql, postgres, mysql, mongodb, redis, query optimization, indexing, schema, migration, orm, sqlite, dynamodb, cassandra, connection pool, transaction

Boundary probes test against: devops, frontend

### devops

Keywords: devops, ci/cd, pipeline, docker, kubernetes, k8s, terraform, ansible, infrastructure, deployment, helm, github actions, gitlab ci, jenkins, argocd, container

Boundary probes test against: frontend, databases

### security

Keywords: security, authentication, authorization, oauth, jwt, encryption, vulnerability, penetration, owasp, cors, csrf, xss, rbac, sso, zero trust, secrets management, tls, certificate, firewall, audit log

Boundary probes test against: databases, devops

### testing

Keywords: testing, test, unit test, integration test, e2e, coverage, tdd, bdd, cypress, playwright, jest, pytest, vitest, test fixture, mock, stub, snapshot test, load test, regression test

Boundary probes test against: architecture, backend

### architecture

Keywords: architecture, system design, design pattern, microservices, monolith, event sourcing, cqrs, domain-driven, hexagonal, clean architecture, solid, api gateway, service mesh, saga pattern

Boundary probes test against: backend, databases

### distributed_systems

Keywords: distributed, consensus, replication, partition, raft, paxos, eventual consistency, message queue, kafka, event-driven, pub/sub, rabbitmq, nats, grpc streaming, load balancing, circuit breaker

Boundary probes test against: frontend, databases

### mobile

Keywords: mobile, ios, android, react native, flutter, swift, kotlin, xcode, app store, google play, push notification, deep link, mobile ui

Boundary probes test against: backend, security

### ml_ai

Keywords: machine learning, deep learning, neural network, training, inference, pytorch, tensorflow, transformer, fine-tuning, rag, embedding, llm, prompt engineering, classification, regression, nlp, computer vision, reinforcement learning, diffusion model, vector database

Boundary probes test against: distributed_systems, api_design

### data_science

Keywords: data science, data analysis, pandas, numpy, jupyter, visualization, statistics, data pipeline, etl, data warehouse, spark, airflow, dbt, feature engineering, a/b test, experiment, dashboard, data lake, bigquery, snowflake, redshift

Boundary probes test against: distributed_systems, devops

### cloud

Keywords: aws, azure, gcp, cloud, s3, ec2, lambda, serverless, cloud function, cloud run, iam, vpc, cdn, route 53, cloudfront, load balancer, auto scaling, fargate, ecs, cloud formation

Boundary probes test against: frontend, databases

### observability

Keywords: observability, monitoring, logging, tracing, metrics, prometheus, grafana, datadog, opentelemetry, alerting, sli, slo, sla, incident, on-call, pagerduty, kibana, elasticsearch, apm

Boundary probes test against: frontend, databases

### api_design

Keywords: api design, openapi, swagger, rest api, api versioning, rate limiting, pagination, hateoas, api gateway, webhook, idempotent, api contract, protobuf, schema registry, backward compatible

Boundary probes test against: devops, ml_ai

## Non-Technical

### legal

Keywords: legal, law, regulation, compliance, contract, liability, intellectual property, gdpr, hipaa, terms of service, privacy policy, copyright, patent

Boundary probes test against: distributed_systems, security

### medical

Keywords: medical, clinical, diagnosis, treatment, patient, pharmacology, symptom, dosage, contraindication, clinical trial, healthcare, therapeutic

Boundary probes test against: ml_ai, security

### financial

Keywords: financial, accounting, revenue, profit, balance sheet, investment, portfolio, tax, audit, budgeting, financial model, valuation, equity, debt, forex

Boundary probes test against: data_science, security

### writing

Keywords: writing, copywriting, content, blog, article, editorial, prose, narrative, technical writing, documentation, style guide, tone of voice

Boundary probes test against: databases, security

## Generic probes

Every agent receives three generic out-of-scope probes regardless of domain:

1. Time-sensitive factual question (Federal Reserve interest rate)
2. Medical question (warfarin drug interactions)
3. Legal question (GPL licensing in proprietary products)

These verify that agents hedge appropriately on questions that fall outside any technical domain.

## Contributing new domains

To add a new domain:

1. Check this file first. New domains must not duplicate or conflict with existing domains. If an existing domain already covers your use case, consider adding keywords to it instead of creating a new domain.
2. Add keywords to `DomainKeywords` in `internal/analysis/domains.go`.
3. Add 3-4 boundary probe questions to `BoundaryQuestions` in `internal/probes/questions.go`.
4. Include at least one in-domain calibration question (where the `domain` field matches the domain key) and at least two cross-domain boundary questions.
5. Run `go test ./...` to verify nothing breaks.
6. Update this file with the new domain, its keywords, and which domains its probes test against.

Probe questions should be specific enough that a knowledgeable agent could answer them, but clearly outside scope for agents in other domains. Avoid questions that are trivially googlable or that overlap heavily with multiple domains.
