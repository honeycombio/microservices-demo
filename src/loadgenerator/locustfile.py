#!/usr/bin/python
import random
from locust import HttpUser, task, between

products = [
    '16BEE20109',
    '999SLO4D20',
    'OLJCESPC7Z',
    '66VCHSJNUP',
    '1YMWWN1N4O',
    'DG9ZAG9RCG',
    'L9ECAV7KIM',
    '2ZYFJ3GM2N',
    '0PUK6V6EV0',
    'LS4PSXUNUM',
    '9SIQT8TOJO',
    '6E92ZMYYFZ',
]

currencies = ['EUR', 'USD', 'JPY', 'CAD']

people = [
    {
        'email': 'someone@example.com',
        'street_address': '1600 Amphitheatre Parkway',
        'zip_code': '94043',
        'city': 'Mountain View',
        'state': 'CA',
        'country': 'United States',
        'credit_card_number': '4432-8015-6152-0454',
        'credit_card_expiration_month': '1',
        'credit_card_expiration_year': '2029',
        'credit_card_cvv': '672',
    },
    {
        'email': 'anyone@sample.com',
        'street_address': '410 Terry Avenue North',
        'zip_code': '98109',
        'city': 'Seattle',
        'state': 'WA',
        'country': 'United States',
        'credit_card_number': '4452-7643-1892-6453',
        'credit_card_expiration_month': '3',
        'credit_card_expiration_year': '2027',
        'credit_card_cvv': '397',
    },
    {
        'email': 'aperson@acompany.com',
        'street_address': 'One Microsoft Way',
        'zip_code': '98052',
        'city': 'Redmond',
        'state': 'WA',
        'country': 'United States',
        'credit_card_number': '4582-5783-3465-4667',
        'credit_card_expiration_month': '11',
        'credit_card_expiration_year': '2026',
        'credit_card_cvv': '784',
    },
    {
        'email': 'another@thing.com',
        'street_address': '1 Apple Park Way',
        'zip_code': '95014',
        'city': 'Cupertino',
        'state': 'CA',
        'country': 'United States',
        'credit_card_number': '4104-6732-9834-0990',
        'credit_card_expiration_month': '7',
        'credit_card_expiration_year': '2027',
        'credit_card_cvv': '649',
    },
    {
        'email': 'foo@bar.com',
        'street_address': '1 Hacker Way',
        'zip_code': '94025',
        'city': 'Menlo Park',
        'state': 'CA',
        'country': 'United States',
        'credit_card_number': '4456-7843-4578-8943',
        'credit_card_expiration_month': '8',
        'credit_card_expiration_year': '2029',
        'credit_card_cvv': '835',
    },
]


class WebsiteUser(HttpUser):
    wait_time = between(1, 10)

    @task(1)
    def index(self):
        self.client.get("/")

    @task(2)
    def set_currency(self):
        self.client.post("/setCurrency", {
            'currency_code': random.choice(currencies)})

    @task(10)
    def browse_product(self):
        product = random.choice(products)
        rnd = random.random()
        if rnd >= 0.98:
            product = "BREAKMENOW"

        self.client.get("/product/" + product)

    @task(3)
    def view_cart(self):
        self.client.get("/cart")

    @task(2)
    def add_to_cart(self):
        product = random.choice(products)
        self.client.get("/product/" + product)
        self.client.post("/cart", {
            'product_id': product,
            'quantity': random.choice([1, 2, 3, 4, 5, 10])})

    @task(5)
    def checkout(self):
        self.add_to_cart()
        self.client.post("/cart/checkout", random.choice(people))
