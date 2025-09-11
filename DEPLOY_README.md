# USDTMore PostgreSQL Deployment Guide

## Overview

USDTMore has been updated to use PostgreSQL as the default and only database option. This guide provides simplified deployment instructions for the new PostgreSQL-only configuration.

## Quick Start

### 1. Prerequisites

- Docker and Docker Compose installed
- Git (to clone the repository)

### 2. Environment Configuration

1. Copy the environment template:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` file with your configuration:
   ```bash
   # Required PostgreSQL settings
   DB_PASSWORD=your_secure_password_here
   
   # Required Telegram Bot settings
   TG_BOT_TOKEN=your_bot_token_here
   TG_BOT_ADMIN_ID=your_admin_id_here
   
   # Required API Keys
   ETHERSCAN_API_KEY=your_etherscan_api_key
   TRON_SCAN_API_KEY=your_tron_scan_api_key
   SOLANA_API_KEY=your_solana_api_key
   
   # Application settings
   AUTH_TOKEN=your_auth_token
   APP_URI=https://your-domain.com
   ```

### 3. Deployment Options

#### Option 1: Using the deployment script (Recommended)

```bash
# Make the script executable
chmod +x deploy.sh

# Deploy the application
./deploy.sh deploy

# Check status
./deploy.sh status

# View logs
./deploy.sh logs

# Stop services
./deploy.sh stop
```

#### Option 2: Using Docker Compose directly

```bash
# Start all services
docker-compose up -d

# Check service status
docker-compose ps

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## Service Architecture

The new deployment includes:

1. **PostgreSQL Database** (`postgres`)
   - Port: 5432
   - Database: usdtmore
   - User: usdtmore_user
   - Health checks enabled
   - Persistent data storage

2. **USDTMore Application** (`usdtmore`)
   - Port: 6080
   - Depends on PostgreSQL being healthy
   - Environment-based configuration
   - Log rotation enabled

3. **Redis Cache** (`redis`) - Optional
   - Port: 6379
   - Memory optimization
   - Persistence enabled

## Key Changes from Previous Versions

### Removed Features
- ❌ SQLite database support
- ❌ SQLite-related volume mappings
- ❌ Database type selection (DB_TYPE is now always postgresql)

### Added Features
- ✅ PostgreSQL as the only database option
- ✅ Enhanced health checks for PostgreSQL
- ✅ PostgreSQL client tools in Docker images
- ✅ Optimized connection pooling
- ✅ Production-ready logging configuration
- ✅ Automated deployment script

## Deployment Commands

### Basic Operations

```bash
# Deploy application
./deploy.sh deploy

# Stop application
./deploy.sh stop

# Restart application
./deploy.sh restart

# Check status
./deploy.sh status
```

### Monitoring and Maintenance

```bash
# View all logs
./deploy.sh logs

# View specific service logs
./deploy.sh logs postgres
./deploy.sh logs usdtmore
./deploy.sh logs redis

# Create database backup
./deploy.sh backup

# Complete cleanup (removes all data)
./deploy.sh cleanup
```

### Manual Docker Commands

```bash
# Start services
docker-compose up -d

# Stop services
docker-compose down

# View logs
docker-compose logs -f usdtmore

# Execute commands in PostgreSQL
docker-compose exec postgres psql -U usdtmore_user -d usdtmore

# Check database connection
docker-compose exec usdtmore curl -f http://localhost:6080/
```

## Health Checks

All services include comprehensive health checks:

- **PostgreSQL**: `pg_isready` command
- **USDTMore**: HTTP endpoint check
- **Redis**: `ping` command

## Volume Management

The deployment uses the following Docker volumes:

- `postgres_data`: PostgreSQL database files
- `redis_data`: Redis persistence files
- `usdtmore_logs`: Application logs
- `usdtmore_data`: Application data files

## Network Configuration

All services communicate through the `usdtmore_network` bridge network for secure internal communication.

## Troubleshooting

### Common Issues

1. **PostgreSQL connection failed**
   ```bash
   # Check PostgreSQL service status
   docker-compose logs postgres
   
   # Verify environment variables
   docker-compose config
   ```

2. **Application startup fails**
   ```bash
   # Check application logs
   ./deploy.sh logs usdtmore
   
   # Verify database is healthy
   docker-compose ps
   ```

3. **Port conflicts**
   ```bash
   # Check if ports are in use
   netstat -tulpn | grep -E ":(5432|6080|6379)"
   
   # Modify ports in docker-compose.yml if needed
   ```

### Log Locations

- Application logs: Docker volume `usdtmore_logs`
- PostgreSQL logs: Available via `docker-compose logs postgres`
- Redis logs: Available via `docker-compose logs redis`

## Migration from SQLite

If migrating from a previous SQLite-based installation:

1. Use the provided migration script: `scripts/migrate_to_postgresql.sql`
2. Export data from SQLite and import to PostgreSQL
3. Update configuration to use PostgreSQL connection settings

## Production Considerations

For production deployments:

1. Use strong passwords for `DB_PASSWORD`
2. Configure proper SSL certificates
3. Set up regular backups using `./deploy.sh backup`
4. Monitor logs and service health
5. Consider using the `docker-compose.production.yml` for high-availability setup

## Support

For issues and questions:
- Check the application logs first
- Review the Docker Compose configuration
- Ensure all required environment variables are set
- Verify network connectivity between services