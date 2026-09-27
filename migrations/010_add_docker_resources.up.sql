ALTER TABLE tests ADD COLUMN docker_memory_mb INTEGER;
ALTER TABLE tests ADD COLUMN docker_cpus NUMERIC;
ALTER TABLE tests ADD COLUMN docker_network_enabled BOOLEAN DEFAULT false;
