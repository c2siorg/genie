#!/usr/bin/env python3
"""
Generate a large synthetic transaction CSV (~2000 rows) with realistic
financial data and intentionally injected anomalies for testing the
Genie AI Agent workflow.
"""

import random
import os
import pandas as pd
from datetime import datetime, timedelta

random.seed(42)

# --- Configuration ---
NUM_ROWS = 2000
OUTPUT_DIR = os.path.join(os.path.dirname(__file__), "..", "data")
OUTPUT_FILE = os.path.join(OUTPUT_DIR, "transactions.csv")

# Realistic category definitions with (description_pool, amount_range)
CATEGORIES = {
    "Income": {
        "descriptions": [
            "TechCorp Salary", "FreeLance Payment", "Consulting Invoice",
            "Bonus Payout", "Stock Dividend", "Side Gig Revenue"
        ],
        "amount_range": (2500.0, 8000.0),
        "is_income": True,
    },
    "Groceries": {
        "descriptions": [
            "Whole Foods Market", "Trader Joes", "Costco Wholesale",
            "Safeway", "Kroger", "Aldi Supermarket", "Walmart Grocery"
        ],
        "amount_range": (25.0, 250.0),
    },
    "Food Delivery": {
        "descriptions": [
            "UberEats", "DoorDash", "GrubHub", "Postmates",
            "Instacart Delivery", "Swiggy", "Zomato"
        ],
        "amount_range": (12.0, 75.0),
    },
    "Subscription": {
        "descriptions": [
            "Netflix", "Spotify Premium", "Adobe Creative Cloud",
            "Unused Magazine Sub", "Local Gym", "YouTube Premium",
            "iCloud Storage", "Xbox Game Pass", "Audible",
            "NY Times Digital", "LinkedIn Premium"
        ],
        "amount_range": (5.0, 60.0),
    },
    "Dining": {
        "descriptions": [
            "Starbucks", "Chipotle", "Olive Garden", "Local Cafe",
            "Sushi House", "Pizza Palace", "Thai Express", "McDonalds"
        ],
        "amount_range": (8.0, 120.0),
    },
    "Rent": {
        "descriptions": ["Monthly Rent Payment", "Apartment Lease"],
        "amount_range": (1200.0, 2500.0),
    },
    "Utilities": {
        "descriptions": [
            "Electric Bill", "Water Bill", "Gas Bill",
            "Internet Service", "Phone Bill", "Trash Collection"
        ],
        "amount_range": (30.0, 200.0),
    },
    "Entertainment": {
        "descriptions": [
            "Movie Tickets", "Concert Tickets", "Theme Park",
            "Bowling Night", "Museum Visit", "Comedy Show",
            "Escape Room", "Mini Golf"
        ],
        "amount_range": (15.0, 150.0),
    },
    "Transport": {
        "descriptions": [
            "Uber Ride", "Lyft Ride", "Gas Station Fill-Up",
            "Parking Garage", "Metro Card Reload", "Toll Payment",
            "Car Wash", "Auto Insurance Monthly"
        ],
        "amount_range": (5.0, 120.0),
    },
    "Electronics": {
        "descriptions": [
            "Apple Store", "Best Buy", "Amazon Electronics",
            "Micro Center", "B&H Photo"
        ],
        "amount_range": (50.0, 800.0),
    },
    "Healthcare": {
        "descriptions": [
            "Pharmacy CVS", "Doctor Visit Copay", "Dental Checkup",
            "Vision Care", "Lab Test Fee"
        ],
        "amount_range": (20.0, 300.0),
    },
    "Shopping": {
        "descriptions": [
            "Amazon Purchase", "Target", "IKEA Furniture",
            "Nike Store", "Nordstrom", "TJ Maxx"
        ],
        "amount_range": (15.0, 400.0),
    },
}

# Category weights (how often each category appears)
CATEGORY_WEIGHTS = {
    "Income": 0.04,
    "Groceries": 0.15,
    "Food Delivery": 0.12,
    "Subscription": 0.10,
    "Dining": 0.12,
    "Rent": 0.02,
    "Utilities": 0.05,
    "Entertainment": 0.08,
    "Transport": 0.10,
    "Electronics": 0.04,
    "Healthcare": 0.06,
    "Shopping": 0.12,
}


def random_date(start: datetime, end: datetime) -> datetime:
    delta = end - start
    random_days = random.randint(0, delta.days)
    return start + timedelta(days=random_days)


def generate_normal_transaction(date: datetime) -> dict:
    categories = list(CATEGORY_WEIGHTS.keys())
    weights = list(CATEGORY_WEIGHTS.values())
    category = random.choices(categories, weights=weights, k=1)[0]

    cat_info = CATEGORIES[category]
    description = random.choice(cat_info["descriptions"])
    low, high = cat_info["amount_range"]
    amount = round(random.uniform(low, high), 2)

    return {
        "date": date.strftime("%Y-%m-%d"),
        "description": description,
        "amount": amount,
        "category": category,
    }


def inject_anomalies(transactions: list) -> list:
    """Inject obvious anomalies for both rule-based and Isolation Forest detection."""
    anomalies = []
    start = datetime(2023, 1, 1)
    end = datetime(2024, 12, 31)

    # --- Large spike anomalies (> $500 threshold for rule-based) ---
    spike_descriptions = [
        ("Suspicious Wire Transfer", "Shopping", 4500.0, 9500.0),
        ("Luxury Watch Purchase", "Shopping", 2000.0, 5000.0),
        ("Emergency Repair Service", "Utilities", 1500.0, 3000.0),
        ("Unknown International Charge", "Shopping", 800.0, 3500.0),
        ("Crypto Exchange Deposit", "Electronics", 3000.0, 7000.0),
    ]
    for desc, cat, low, high in spike_descriptions:
        for _ in range(random.randint(2, 4)):
            anomalies.append({
                "date": random_date(start, end).strftime("%Y-%m-%d"),
                "description": desc,
                "amount": round(random.uniform(low, high), 2),
                "category": cat,
            })

    # --- Duplicate subscription anomalies ---
    dup_subs = ["Netflix", "Unused Magazine Sub", "Local Gym"]
    for sub in dup_subs:
        for month in range(1, 25):
            year = 2023 if month <= 12 else 2024
            m = month if month <= 12 else month - 12
            # Double charge same month
            for _ in range(2):
                anomalies.append({
                    "date": datetime(year, m, random.randint(1, 28)).strftime("%Y-%m-%d"),
                    "description": sub,
                    "amount": round(random.uniform(10.0, 50.0), 2),
                    "category": "Subscription",
                })

    # --- Micro-transaction anomalies (unusually tiny) ---
    for _ in range(15):
        anomalies.append({
            "date": random_date(start, end).strftime("%Y-%m-%d"),
            "description": "Unknown Micro Charge",
            "amount": round(random.uniform(0.01, 0.50), 2),
            "category": "Shopping",
        })

    return anomalies


def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    start_date = datetime(2023, 1, 1)
    end_date = datetime(2024, 12, 31)

    # Generate normal transactions
    normal_count = NUM_ROWS - 200  # Reserve room for anomalies
    transactions = []
    for _ in range(normal_count):
        date = random_date(start_date, end_date)
        transactions.append(generate_normal_transaction(date))

    # Inject anomalies
    anomaly_rows = inject_anomalies(transactions)
    transactions.extend(anomaly_rows)

    # Shuffle and sort by date
    random.shuffle(transactions)
    transactions.sort(key=lambda t: t["date"])

    df = pd.DataFrame(transactions)
    df.to_csv(OUTPUT_FILE, index=False)

    print(f"Generated {len(df)} transactions -> {os.path.abspath(OUTPUT_FILE)}")
    print(f"  Date range: {df['date'].min()} to {df['date'].max()}")
    print(f"  Categories: {df['category'].nunique()}")
    print(f"  Total amount: ${df['amount'].sum():,.2f}")
    print(f"\nCategory breakdown:")
    print(df.groupby("category")["amount"].agg(["count", "sum", "mean"]).to_string())


if __name__ == "__main__":
    main()
