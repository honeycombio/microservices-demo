#!/usr/bin/python
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import random
from locust import HttpUser, TaskSet, between

products = [
    '0PUK6V6EV0',
    '1YMWWN1N4O',
    '2ZYFJ3GM2N',
    '66VCHSJNUP',
    '6E92ZMYYFZ',
    '9SIQT8TOJO',
    'L9ECAV7KIM',
    'LS4PSXUNUM',
    'OLJCESPC7Z',
    'BREAKMENOW']


def index(l):
    l.client.get("/")


def set_currency(l):
    currencies = ['EUR', 'USD', 'JPY', 'CAD']
    l.client.post("/setCurrency",
                  {'currency_code': random.choice(currencies)})


def browse_product(l):
    l.client.get("/product/" + random.choice(products))


def view_cart(l):
    l.client.get("/cart")


def add_to_cart(l):
    product = random.choice(products)
    l.client.get("/product/" + product)
    l.client.post("/cart", {
        'product_id': product,
        'quantity': random.choice([1, 2, 3, 4, 5, 10])})


def checkout(l):
    add_to_cart(l)
    l.client.post("/cart/checkout", {
        'email': 'someone@example.com',
        'street_address': '1600 Amphitheatre Parkway',
        'zip_code': '94043',
        'city': 'Mountain View',
        'state': 'CA',
        'country': 'United States',
        'credit_card_number': '4432-8015-6152-0454',
        'credit_card_expiration_month': '1',
        'credit_card_expiration_year': '2039',
        'credit_card_cvv': '672',
    })


class UserBehavior(TaskSet):

    def on_start(self):
        index(self)

    tasks = {index: 1,
             set_currency: 2,
             browse_product: 10,
             add_to_cart: 2,
             view_cart: 3,
             checkout: 5}


class WebsiteUser(HttpUser):
    tasks = [UserBehavior]
    wait_time = between(1, 10)
