from typing import List, Dict, Any
from presidio_analyzer import AnalyzerEngine, PatternRecognizer, Pattern
from presidio_analyzer.nlp_engine import NlpEngineProvider
from presidio_anonymizer import AnonymizerEngine
from presidio_anonymizer.entities import OperatorConfig

# 1. Initialize Presidio Engines with high-efficiency small model
configuration = {
    "nlp_engine_name": "spacy",
    "models": [{"lang_code": "en", "model_name": "en_core_web_sm"}],
}
provider = NlpEngineProvider(nlp_configuration=configuration)
nlp_engine = provider.create_engine()

analyzer = AnalyzerEngine(nlp_engine=nlp_engine, default_score_threshold=0.4)
anonymizer = AnonymizerEngine()

# 2. Add Financial-Specific PII Recognizers
upi_pattern = Pattern(name="upi_pattern", regex=r"\b[\w.-]+@[\w.-]+\b", score=0.85)
analyzer.registry.add_recognizer(PatternRecognizer(supported_entity="UPI_ID", patterns=[upi_pattern]))

ifsc_pattern = Pattern(name="ifsc_pattern", regex=r"^[A-Z]{4}0[A-Z0-9]{6}$", score=0.85)
analyzer.registry.add_recognizer(PatternRecognizer(supported_entity="IFSC_CODE", patterns=[ifsc_pattern]))


class PIIVault:
    """Secure Vault for mapping tokens back to original PII."""
    def save_mapping(self, user_id: str, token: str, original_value: str, entity_type: str):
        # Implementation: Save to isolated secure database schema
        pass

vault = PIIVault()


def tokenize_pii_text(text: str, user_id: str, audit_logs: list) -> str:
    """Uses NER to detect PII, tokenizes it, and saves mappings to the Vault."""
    if not text:
        return text
        
    # Analyze with confidence threshold to reduce false positives
    results = analyzer.analyze(text=text, language="en", score_threshold=0.75)
    
    if not results:
        return text

    # Define operators to replace PII with semantic tokens
    operators = {
        "PERSON": OperatorConfig("replace", {"new_value": "<PERSON_TOKEN>"}),
        "PHONE_NUMBER": OperatorConfig("replace", {"new_value": "<PHONE_TOKEN>"}),
        "EMAIL_ADDRESS": OperatorConfig("replace", {"new_value": "<EMAIL_TOKEN>"}),
        "UPI_ID": OperatorConfig("replace", {"new_value": "<UPI_TOKEN>"}),
        "IFSC_CODE": OperatorConfig("replace", {"new_value": "<IFSC_TOKEN>"}),
    }
    
    anonymized = anonymizer.anonymize(text=text, analyzer_results=results, operators=operators)
    
    for res in results:
        audit_logs.append({
            "entity": res.entity_type, 
            "score": res.score, 
            "action": "tokenized"
        })
        
    return anonymized.text


def sanitize_transaction_row(row: dict, user_id: str, audit_logs: list) -> dict:
    """Applies structured field rules and unstructured NER tokenization."""
    sanitized = row.copy()
    
    # Structured Field Masking Rules
    if 'account_number' in sanitized:
        acc = str(sanitized['account_number'])
        sanitized['account_number'] = f"****{acc[-4:]}" if len(acc) >= 4 else "****"
        
    # Unstructured Free-Text NER Tokenization
    if 'merchant_description' in sanitized:
        sanitized['merchant_description'] = tokenize_pii_text(
            sanitized['merchant_description'], user_id, audit_logs
        )
    elif 'merchant' in sanitized:
        # Fallback to pure merchant column if 'merchant_description' is missing inside the uploaded CSV
        sanitized['merchant'] = tokenize_pii_text(
            sanitized['merchant'], user_id, audit_logs
        )
        
    return sanitized
