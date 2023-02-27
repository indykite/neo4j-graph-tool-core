CREATE CONSTRAINT unique_schema_id ON (n:Schema) ASSERT n.id IS UNIQUE;
