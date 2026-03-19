import pandas as pd
import datetime
import numpy as np
from storage.state import SystemState
from telemetry.observability import trace_execution

def forecast_with_arima(monthly_df: pd.DataFrame, periods: int = 6) -> dict:
    """
    Forecast future values using an adaptive ARIMA model.
    Falls back to non-seasonal if data is insufficient (<36 months).
    """
    import pmdarima as pm

    df = monthly_df.copy()
    df["ds"] = pd.to_datetime(df["ds"])
    y = df["y"].values

    # Seasonal ARIMA needs at least 3-4 cycles (36-48 months) for stability
    use_seasonal = len(y) >= 36
    
    try:
        model = pm.auto_arima(
            y,
            seasonal=use_seasonal,
            m=12 if use_seasonal else 1,
            trace=False,
            suppress_warnings=True,
            stepwise=True,
            error_action="ignore",
        )
        preds = model.predict(n_periods=periods)
        
        last_date = df["ds"].iloc[-1]
        future_dates = pd.date_range(
            start=last_date + pd.offsets.MonthEnd(1),
            periods=periods,
            freq="ME"
        )
        
        forecast_df = pd.DataFrame({
            "ds": future_dates,
            "yhat": preds
        })
        return {"model": model, "forecast_df": forecast_df}
    except Exception as e:
        return {"error": str(e)}


def forecast_with_prophet(monthly_df: pd.DataFrame, periods: int = 6) -> dict:
    """Forecast future values using Facebook Prophet."""
    from prophet import Prophet
    import logging
    
    # Disable prophet logs
    logger = logging.getLogger('cmdstanpy')
    logger.addHandler(logging.NullHandler())
    logger.propagate = False
    logger.setLevel(logging.CRITICAL)

    try:
        model = Prophet(yearly_seasonality=True, weekly_seasonality=False)
        model.fit(monthly_df)
        
        future = model.make_future_dataframe(periods=periods, freq='ME')
        forecast = model.predict(future)
        
        return {
            "model": model,
            "forecast_df": forecast[["ds", "yhat", "yhat_lower", "yhat_upper"]]
        }
    except Exception as e:
        return {"error": str(e)}


@trace_execution
def cash_flow_forecasting_node(state: SystemState) -> dict:
    """Forecast cash flow using both ARIMA and Prophet models."""
    print("[Forecasting Agent] Analyzing monthly spending trends...")
    
    transactions = state.get("transactions", [])
    if not transactions:
        return {"arima_forecast": "No transactions for forecasting.", "prophet_forecast": "No transactions for forecasting."}

    df = pd.DataFrame(transactions)
    if not all(col in df.columns for col in ["date", "amount"]):
        return {
            "arima_forecast": "Data source is missing required forecasting columns.",
            "prophet_forecast": "Data source is missing required forecasting columns."
        }

    df["date"] = pd.to_datetime(df["date"])
    
    # Filter to expenses
    expenses_df = df[df["amount"] > 0].copy()
    if expenses_df.empty:
        return {"arima_forecast": "No expenses found for forecasting.", "prophet_forecast": "No expenses found for forecasting."}

    # Aggregate by month
    expenses_df.set_index("date", inplace=True)
    monthly_series = expenses_df["amount"].resample("ME").sum().reset_index()
    monthly_series.columns = ["ds", "y"]
    
    if len(monthly_series) < 5:
        return {
            "arima_forecast": "Insufficient monthly data history (need 5+ months).",
            "prophet_forecast": "Insufficient monthly data history (need 5+ months)."
        }

    # 1. ARIMA
    print("   -> Running ARIMA model...")
    try:
        arima_res = forecast_with_arima(monthly_series)
    except Exception as e:
        arima_res = {"error": str(e)}
    
    if "error" in arima_res:
        arima_str = f"ARIMA forecast failed: {arima_res['error']}"
    else:
        fdf = arima_res["forecast_df"]
        arima_str = "ARIMA Forecast (next 6 months):\n"
        for _, row in fdf.iterrows():
            arima_str += f"      {row['ds'].strftime('%Y-%m')}: ${row['yhat']:,.2f}\n"
        arima_str += f"      Average predicted monthly expense: ${fdf['yhat'].mean():,.2f}"

    # 2. Prophet
    print("   -> Running Prophet model...")
    try:
        prophet_res = forecast_with_prophet(monthly_series)
    except Exception as e:
        prophet_res = {"error": str(e)}

    if "error" in prophet_res:
        prophet_str = f"Prophet forecast failed: {prophet_res['error']}"
    else:
        fdf = prophet_res["forecast_df"].tail(6) # Get future only
        prophet_str = "Prophet Forecast (next 6 months):\n"
        for _, row in fdf.iterrows():
            prophet_str += f"      {row['ds'].strftime('%Y-%m')}: ${row['yhat']:,.2f} (range: ${row['yhat_lower']:,.2f} - ${row['yhat_upper']:,.2f})\n"
        prophet_str += f"      Average predicted monthly expense: ${fdf['yhat'].mean():,.2f}"

    print("   -> Cash flow forecasting complete.")
    return {
        "arima_forecast": arima_str,
        "prophet_forecast": prophet_str
    }