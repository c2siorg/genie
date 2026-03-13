from sentence_transformers import SentenceTransformer

# Load the model once when the worker process boots up.
# all-MiniLM-L6-v2 is standard, lightweight, and produces 384-dimensional embeddings.
model = SentenceTransformer('all-MiniLM-L6-v2')

def generate_embedding(semantic_text: str) -> list[float]:
    """Generates a 384-dimensional vector embedding for the given text."""
    # The numpy array needs to be converted back to a list of floats for pgvector
    embedding = model.encode(semantic_text)
    return embedding.tolist()
