CREATE CONSTRAINT unique_session_id ON (n:Session) ASSERT n.id IS UNIQUE;
