"""Airflow DAG that processes customer orders and emits demo invoices."""

from __future__ import annotations

import json
import logging
import time
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List

import pendulum
from airflow.decorators import dag, task

LOGGER = logging.getLogger(__name__)

ORDERS_PATH = Path("/opt/airflow/data/orders.json")
INVOICE_DIR = Path("/opt/airflow/data/invoices")

REQUIRED_ORDER_KEYS = {
    "order_id",
    "customer_email",
    "customer_address",
    "country",
    "tax_percent",
    "items",
    "subtotal",
    "tax_amount",
    "total_charged",
}


def _validate_order(order: Dict[str, Any], index: int) -> None:
    missing = REQUIRED_ORDER_KEYS - set(order)
    if missing:
        raise ValueError(f"Order at index {index} missing required keys: {sorted(missing)}")

    if not isinstance(order.get("items"), list):
        raise ValueError(f"Order {order.get('order_id', index)} has invalid items payload.")


def _build_invoice(order: Dict[str, Any]) -> Dict[str, Any]:
    subtotal_recalc = sum(item["unit_price"] * item["quantity"] for item in order["items"])
    tax_from_rate = round(subtotal_recalc * (order["tax_percent"] / 100), 2)
    total_recalc = round(subtotal_recalc + tax_from_rate, 2)

    return {
        "order": order,
        "invoice_generated_at": datetime.utcnow().isoformat() + "Z",
        "computation_summary": {
            "subtotal_from_items": round(subtotal_recalc, 2),
            "tax_from_rate": tax_from_rate,
            "total_with_tax": total_recalc,
        },
    }


@dag(
    dag_id="process_orders_dag",
    schedule="@hourly",
    start_date=pendulum.datetime(2024, 1, 1, tz="UTC"),
    catchup=False,
    default_args={"owner": "airflow"},
    description="Process incoming orders, generate invoices, and send customer notifications.",
    tags=["orders", "invoices", "demo"],
)
def process_orders_dag():
    @task()
    def read_orders() -> List[Dict[str, Any]]:
        LOGGER.info("read_orders: loading orders from %s", ORDERS_PATH)
        with ORDERS_PATH.open("r", encoding="utf-8") as handle:
            payload = json.load(handle)

        orders = payload.get("orders", [])
        if not isinstance(orders, list):
            raise ValueError("orders.json must contain an array under the 'orders' key.")

        LOGGER.info("read_orders: payload contains %d orders", len(orders))
        for idx, order in enumerate(orders):
            _validate_order(order, idx)
            LOGGER.debug(
                "read_orders: order %s (customer=%s, total=%.2f) passed validation",
                order["order_id"],
                order["customer_email"],
                order["total_charged"],
            )

        LOGGER.info("read_orders: validation complete for %d orders", len(orders))
        return orders

    @task()
    def generate_invoices(orders: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        INVOICE_DIR.mkdir(parents=True, exist_ok=True)
        LOGGER.info(
            "generate_invoices: starting with %d orders; output dir %s",
            len(orders),
            INVOICE_DIR,
        )

        invoice_metadata: List[Dict[str, Any]] = []

        for order in orders:
            invoice_payload = _build_invoice(order)
            invoice_path = INVOICE_DIR / f"invoice_{order['order_id']}.json"

            with invoice_path.open("w", encoding="utf-8") as handle:
                json.dump(invoice_payload, handle, indent=2)

            LOGGER.info(
                "generate_invoices: wrote invoice for order %s (customer=%s, total=%.2f) to %s",
                order["order_id"],
                order["customer_email"],
                order["total_charged"],
                invoice_path,
            )
            LOGGER.debug(
                "generate_invoices: computation_summary=%s",
                invoice_payload["computation_summary"],
            )

            invoice_metadata.append(
                {
                    "order_id": order["order_id"],
                    "customer_email": order["customer_email"],
                    "invoice_path": str(invoice_path),
                    "total_charged": order["total_charged"],
                }
            )

        LOGGER.info(
            "generate_invoices: finished creating %d invoice artifacts",
            len(invoice_metadata),
        )
        return invoice_metadata

    @task()
    def email_invoices(invoice_metadata: List[Dict[str, Any]]) -> None:
        LOGGER.info(
            "email_invoices: preparing to send %d invoices", len(invoice_metadata)
        )
        for metadata in invoice_metadata:
            LOGGER.info(
                "email_invoices: sending invoice for order %s to %s (total=%.2f, path=%s)",
                metadata["order_id"],
                metadata["customer_email"],
                metadata["total_charged"],
                metadata["invoice_path"],
            )
            LOGGER.debug(
                "email_invoices: payload=%s", metadata,
            )
            # Simulate latency to mimic an external API call.
            time.sleep(0.1)

        LOGGER.info("email_invoices: completed sending %d invoices", len(invoice_metadata))

    orders_payload = read_orders()
    LOGGER.info("process_orders_dag: scheduled generate_invoices task")
    invoice_info = generate_invoices(orders_payload)
    LOGGER.info("process_orders_dag: scheduled email_invoices task")
    email_invoices(invoice_info)
    LOGGER.info("process_orders_dag: DAG wiring complete")


dag_instance = process_orders_dag()
