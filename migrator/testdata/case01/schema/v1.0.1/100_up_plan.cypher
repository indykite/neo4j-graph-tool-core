CREATE CONSTRAINT unique_plan_id ON (n:Plan) ASSERT n.id IS UNIQUE;
