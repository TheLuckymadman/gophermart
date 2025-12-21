CREATE TABLE users (
    id serial PRIMARY KEY,
    login text UNIQUE NOT NULL,
    password text NOT NULL,
    balance bigint DEFAULT 0,
    withdrawn bigint DEFAULT 0,
    created_at timestamp DEFAULT now()
);
CREATE TABLE orders (
    id serial PRIMARY KEY,
    number text NOT NULL UNIQUE,
    status text NOT NULL,
    accrual bigint DEFAULT 0,
    accrual_status text,
    retry_cnt integer DEFAULT 0,
    created_at timestamp DEFAULT now(),
    processed_at timestamp,
    user_id integer NOT NULL REFERENCES users(id)
);
CREATE TABLE withdrawals (
    id serial PRIMARY KEY,
    number text NOT NULL UNIQUE,
    withdrawn bigint NOT NULL,
    created_at timestamp DEFAULT now(),
    user_id integer NOT NULL REFERENCES users(id)
);
CREATE TABLE operations (
    id serial PRIMARY KEY,
    type text NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    amount bigint DEFAULT 0,
    user_id integer NOT NULL,
    order_id integer, 
    withdrawal_id integer,
    CONSTRAINT fk_user
        FOREIGN KEY(user_id)
            REFERENCES users(id),
    CONSTRAINT fk_order
        FOREIGN KEY(order_id)
            REFERENCES orders(id),
    CONSTRAINT fk_withdrawal
        FOREIGN KEY(withdrawal_id)
            REFERENCES withdrawals(id)
);
CREATE INDEX idx_users_login ON users(login);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_operations_user_id ON operations(user_id);
CREATE INDEX idx_orders_accrual_status ON orders(accrual_status);
CREATE INDEX idx_withdrawals_user_id ON withdrawals(user_id);