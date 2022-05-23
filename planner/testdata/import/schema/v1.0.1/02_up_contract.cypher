CREATE CONSTRAINT unique_contract_id ON (n:Contract) ASSERT n.id IS UNIQUE;
