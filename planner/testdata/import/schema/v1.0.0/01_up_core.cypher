CREATE CONSTRAINT unique_dataversion_version ON (n:DataVersion) ASSERT n.version IS UNIQUE;
CREATE CONSTRAINT unique_modelversion_version ON (n:ModelVersion) ASSERT n.version IS UNIQUE;

CREATE CONSTRAINT unique_movie_name ON (m:Movie) ASSERT m.name IS UNIQUE;
