#!/bin/bash
# filepath: /Users/tounilab/Workspace/tounilab.com/fabric/dockerfiles/entrypoint-sqlserver.sh

# Start SQL Server in background
/opt/mssql/bin/sqlservr &
SERVER_PID=$!

# Wait for SQL Server to be ready (up to 60 seconds)
echo "Waiting for SQL Server to start..."
for i in {1..60}; do
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P TestPassword123! -C -Q "SELECT 1" &>/dev/null
    if [ $? -eq 0 ]; then
        echo "SQL Server is ready!"
        break
    fi
    echo "Attempt $i: Waiting..."
    sleep 1
done

# Fix permissions
chown mssql:mssql /tmp/*.sql

# Run init scripts
echo "Running init script..."
/opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P TestPassword123! -C -i /tmp/init.sql

echo "Running seed script..."
/opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P TestPassword123! -C -d test_db -i /tmp/seed.sql

# Keep container running
wait $SERVER_PID