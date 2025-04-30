CREATE TABLE users
(
    id           SERIAL PRIMARY KEY,
    name         VARCHAR(100) NOT NULL UNIQUE,
    access_level VARCHAR(50)  NOT NULL,
    password     VARCHAR(255) NOT NULL
);

CREATE TABLE pocket_money
(
    id               SERIAL PRIMARY KEY,
    receiver_user_id INT  NOT NULL,
    amount           INT  NOT NULL,
    specific_date    DATE NOT NULL,
    confirmed        BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (receiver_user_id) REFERENCES users (id) ON DELETE CASCADE,
    UNIQUE (receiver_user_id, specific_date)
);
