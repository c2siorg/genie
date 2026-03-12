from storage.state import SystemState
import pandas as pd

def cash_flow_forecasting_node(state: SystemState) -> dict:
    """Agentic node to forecast future cash flow based on existing trends."""
    print("[Cash Flow Agent] Forecasting future cash flow...")
    
    transactions = state.get("transactions", [])
    if not transactions:
        return {"cash_flow_prediction": "Insufficient data to predict cash flow."}
        
    total_spent = sum(t.get("amount", 0) for t in transactions)
    # Extremely basic mock prediction for next month
    projected = total_spent * 1.05  # Assume 5% increase based on mock inflation/trend
    
    prediction = f"Based on current velocity, expecting next month's expenses around ${projected:.2f}. "
    prediction += "Ensure your income covers this projected amount to maintain a positive cash flow."
    
    print(" -> Cash flow forecast generated.")
    return {"cash_flow_prediction": prediction}


def forecast_with_prophet(monthly_df: pd.DataFrame, periods: int = 12) -> dict:
    """
    Forecast future values of a time series using the Prophet model.

    This function trains a Prophet model on historical monthly financial data
    (e.g., expenses or cashflow) and generates predictions for future periods.

    Expected Input Format
    ---------------------
    monthly_df : pd.DataFrame
        DataFrame containing time-series data with the following columns:
        - ds : datetime column representing timestamps
        - y  : numeric column representing the observed value (e.g., spending)

    periods : int, optional (default=12)
        Number of future time steps (months) to forecast.

    Returns
    -------
    dict
        {
            "model": Prophet
                Trained Prophet forecasting model.

            "forecast_df": pd.DataFrame
                Full Prophet prediction dataframe containing historical
                predictions and future forecasts.

            "future_forecast": pd.DataFrame
                Dataframe containing only the future forecasted periods
                with columns:
                - ds
                - yhat
                - yhat_lower
                - yhat_upper
        }

    Notes
    -----
    Prophet models time series as a combination of:
        trend + seasonality + noise.

    It works well for financial data that contains:
        - recurring patterns
        - seasonal behavior
        - long-term trends.

    Example
    -------
    >>> result = forecast_with_prophet(monthly_df, periods=6)
    >>> result["future_forecast"]
    """

    try:
        from prophet import Prophet
    except Exception:
        from fbprophet import Prophet

    # Ensure correct format
    df = monthly_df.copy()
    df["ds"] = pd.to_datetime(df["ds"])

    model = Prophet()

    model.fit(df)

    future = model.make_future_dataframe(
        periods=periods,
        freq="M"
    )

    forecast = model.predict(future)

    future_forecast = forecast.tail(periods)[
        ["ds", "yhat", "yhat_lower", "yhat_upper"]
    ].reset_index(drop=True)

    return {
        "model": model,
        "forecast_df": forecast,
        "future_forecast": future_forecast
    }

def forecast_with_arima(monthly_df: pd.DataFrame, periods: int = 12) -> dict:
    """
    Forecast future values of a time series using an ARIMA model.

    This function uses pmdarima's auto_arima to automatically determine
    the best ARIMA configuration and then predicts future time steps.

    Expected Input Format
    ---------------------
    monthly_df : pd.DataFrame
        DataFrame containing time-series data with the following columns:
        - ds : datetime column representing timestamps
        - y  : numeric column representing the observed value

    periods : int, optional (default=12)
        Number of future periods (months) to forecast.

    Returns
    -------
    dict
        {
            "model": ARIMA
                Trained ARIMA model.

            "forecast_df": pd.DataFrame
                Dataframe containing forecasted values for future periods
                with columns:
                - ds
                - yhat
        }

    Notes
    -----
    ARIMA models time series using autoregression and moving averages.

    The model automatically selects optimal parameters:
        ARIMA(p, d, q)

    Seasonal ARIMA (SARIMA) is used when:
        seasonal=True
        m=12  (monthly seasonality)

    Example
    -------
    >>> result = forecast_with_arima(monthly_df, periods=6)
    >>> result["forecast_df"]
    """

    import pmdarima as pm

    df = monthly_df.copy()
    df["ds"] = pd.to_datetime(df["ds"])

    y = df["y"].values

    model = pm.auto_arima(
        y,
        seasonal=True,
        m=12,
        trace=False,
        suppress_warnings=True
    )
    
    preds = model.predict(n_periods=periods)

    last_date = df["ds"].iloc[-1]

    future_dates = pd.date_range(
        start=last_date + pd.offsets.MonthEnd(1),
        periods=periods,
        freq="M"
    )

    forecast_df = pd.DataFrame({
        "ds": future_dates,
        "yhat": preds
    })

    return {
        "model": model,
        "forecast_df": forecast_df
    }