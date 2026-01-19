# FastShip TODO

## Phase 1: Core (DONE ✅)
- [x] Configuration parsing with YAML
- [x] Environment variable overrides
- [x] Webhook authentication (HMAC, simple secret, GitLab)
- [x] HTTP server with Echo
- [x] Deploy endpoint
- [x] Health check system
- [x] Rollback on failure
- [x] Docker Compose integration
- [x] Per-service deployment serialization
- [x] In-memory deployment tracking
- [x] API endpoints (deploy, health, list deployments)

## Phase 2: Persistence (NEXT)
- [ ] Add SQLite database
- [ ] Create schema (deployments, images tables)
- [ ] Implement store package
- [ ] Persist deployment history
- [ ] Track image versions
- [ ] Enable rollback to specific versions
- [ ] Migration system for schema updates

## Phase 3: Web Dashboard
- [ ] Install templ for templates
- [ ] Create layout.templ
- [ ] Create dashboard.templ (overview)
- [ ] Create deployments.templ (history)
- [ ] Add basic authentication
- [ ] Real-time updates with HTMX
- [ ] Manual deploy/rollback buttons
- [ ] Deployment logs view

## Phase 4: Testing
- [ ] Unit tests for config package
- [ ] Unit tests for webhook package
- [ ] Unit tests for deploy package
- [ ] Integration tests for docker package
- [ ] E2E tests for complete flows
- [ ] Test coverage >80%
- [ ] Benchmark tests for performance

## Phase 5: Observability
- [ ] Structured logging (zerolog or zap)
- [ ] Log levels (debug, info, warn, error)
- [ ] Deployment logs storage
- [ ] Prometheus metrics endpoint
- [ ] Metrics: deployments_total, deployment_duration, etc.
- [ ] Health check metrics

## Phase 6: Notifications
- [ ] Slack notifications
- [ ] Discord webhooks
- [ ] Email notifications (SMTP)
- [ ] Custom webhook notifications
- [ ] Notification templates
- [ ] Per-service notification config

## Phase 7: Security & Reliability
- [ ] Rate limiting per IP/service
- [ ] IP whitelisting
- [ ] API key management (per-service keys)
- [ ] Deployment queuing
- [ ] Graceful shutdown
- [ ] Deployment timeout limits
- [ ] Retry failed deployments

## Phase 8: Advanced Features
- [ ] Deployment approvals (manual approve required)
- [ ] Deployment scheduling (deploy at specific time)
- [ ] Blue-green deployments
- [ ] Canary deployments
- [ ] Pre/post deployment hooks
- [ ] Environment-specific configs
- [ ] Multi-node support

## Phase 9: Kubernetes Support
- [ ] kubectl integration
- [ ] Kubernetes deployment support
- [ ] Helm chart deployments
- [ ] K8s health checks
- [ ] K8s rollback

## Phase 10: Developer Experience
- [ ] CLI tool for testing
- [ ] Docker Compose plugin
- [ ] GitHub Action
- [ ] GitLab CI template
- [ ] Terraform provider

## Nice to Have
- [ ] Auto-update from GitHub releases
- [ ] Backup/restore configuration
- [ ] Deployment statistics
- [ ] Cost tracking integration
- [ ] Deployment annotations
- [ ] Custom deployment scripts
- [ ] WebSocket for real-time updates
- [ ] GraphQL API
- [ ] gRPC API

## Documentation
- [ ] API reference (OpenAPI/Swagger)
- [ ] Video tutorials
- [ ] Blog posts
- [ ] Comparison with alternatives
- [ ] Security best practices guide
- [ ] Troubleshooting guide

## Community
- [ ] Contributing guide
- [ ] Code of conduct
- [ ] Issue templates
- [ ] PR templates
- [ ] Roadmap
- [ ] Changelog

## Priority Order
1. **Phase 2**: Persistence - Most important for production use
2. **Phase 4**: Testing - Essential for reliability
3. **Phase 3**: Dashboard - Nice UX improvement
4. **Phase 5**: Observability - Important for debugging
5. **Phase 6**: Notifications - User request driven
6. **Phase 7**: Security - As needed based on usage
7. **Phase 8+**: Advanced features - Future

## Quick Wins (Low Effort, High Value)
- [ ] Add rate limiting (1-2 hours)
- [ ] Add API authentication for /api/deployments (30 min)
- [ ] Add deployment timeout config (1 hour)
- [ ] Add graceful shutdown (1 hour)
- [ ] Improve error messages (2 hours)
- [ ] Add deployment duration tracking (30 min)

## Known Issues
- No authentication on GET /api/deployments
- Rollback endpoint is a stub
- No deployment logs storage
- No rate limiting
- No graceful shutdown
- Deployment history lost on restart

## Research Needed
- Best approach for blue-green deployments
- How to handle multi-container services
- Options for distributed deployment coordination
- Database migration strategies
- Monitoring best practices
