// Evaluation Trace Review Interface
// Manages trace fetching, annotation submission, and keyboard shortcuts

class EvalReviewInterface {
	constructor() {
		this.traces = [];
		this.currentTraceIndex = 0;
		this.currentAnnotation = null;
		this.selectedLabel = null;
		this.annotatorID = this.getOrCreateAnnotatorID();
		this.apiBaseURL = '/v1/eval';

		this.initializeDOM();
		this.attachEventListeners();
		this.attachKeyboardShortcuts();
		this.loadClusters();
		this.loadRubrics();
	}

	// === Initialization ===

	initializeDOM() {
		// Get all required elements
		this.elements = {
			// Controls
			btnLoadTraces: document.getElementById('btnLoadTraces'),
			sampleStrategy: document.getElementById('sampleStrategy'),
			btnPass: document.getElementById('btnPass'),
			btnFail: document.getElementById('btnFail'),
			btnUncertain: document.getElementById('btnUncertain'),
			btnSubmitAnnotation: document.getElementById('btnSubmitAnnotation'),

			// Form inputs
			primaryFailure: document.getElementById('primaryFailure'),
			secondaryFailures: document.getElementById('secondaryFailures'),
			confidenceSlider: document.getElementById('confidenceSlider'),
			confidenceValue: document.getElementById('confidenceValue'),
			notes: document.getElementById('notes'),
			deferCheckbox: document.getElementById('deferCheckbox'),
			deferReason: document.getElementById('deferReason'),

			// Display areas
			traceContent: document.getElementById('traceContent'),
			listItems: document.getElementById('listItems'),
			currentTrace: document.getElementById('currentTrace'),
			annotationStatus: document.getElementById('annotationStatus'),
			failureSection: document.getElementById('failureSection'),
			similarTraces: document.getElementById('similarTraces'),
			rubricsList: document.getElementById('rubricsList'),
			clustersList: document.getElementById('clustersList'),
			footerStatus: document.getElementById('footerStatus'),
			footerProgress: document.getElementById('footerProgress'),

			// Tabs
			tabBtns: document.querySelectorAll('.tab-btn'),
		};

		// Verify critical elements exist
		if (!this.elements.btnLoadTraces) {
			console.error('Required DOM elements missing');
			return;
		}
	}

	attachEventListeners() {
		// Load button
		this.elements.btnLoadTraces.addEventListener('click', () => this.loadTraces());

		// Pass/Fail/Uncertain buttons
		this.elements.btnPass.addEventListener('click', () => this.setLabel('pass'));
		this.elements.btnFail.addEventListener('click', () => this.setLabel('fail'));
		this.elements.btnUncertain.addEventListener('click', () => this.setLabel('uncertain'));

		// Confidence slider
		this.elements.confidenceSlider.addEventListener('input', (e) => {
			this.elements.confidenceValue.textContent = e.target.value + '%';
		});

		// Defer checkbox
		this.elements.deferCheckbox.addEventListener('change', (e) => {
			this.elements.deferReason.style.display = e.target.checked ? 'block' : 'none';
		});

		// Submit annotation
		this.elements.btnSubmitAnnotation.addEventListener('click', () => this.submitAnnotation());

		// Tab switching
		this.elements.tabBtns.forEach((btn) => {
			btn.addEventListener('click', (e) => this.switchTab(e.target.dataset.tab));
		});
	}

	attachKeyboardShortcuts() {
		document.addEventListener('keydown', (e) => {
			if (e.ctrlKey || e.metaKey) return; // Ignore Ctrl+/Cmd+ combinations

			switch (e.key.toUpperCase()) {
				case 'N':
					e.preventDefault();
					this.nextTrace();
					break;
				case 'P':
					e.preventDefault();
					this.previousTrace();
					break;
				case 'S':
					e.preventDefault();
					this.setLabel('pass');
					break;
				case 'F':
					e.preventDefault();
					this.setLabel('fail');
					break;
				case 'D':
					e.preventDefault();
					this.toggleDefer();
					break;
			}
		});
	}

	// === Trace Management ===

	async loadTraces() {
		const sampleStrategy = this.elements.sampleStrategy.value;
		const url = `${this.apiBaseURL}/traces?limit=50&sample=${sampleStrategy}`;

		try {
			this.setStatus('Loading traces...');
			const response = await fetch(url);

			if (!response.ok) {
				throw new Error(`HTTP ${response.status}`);
			}

			const data = await response.json();
			this.traces = data.traces || [];

			this.setStatus(`Loaded ${this.traces.length} traces`);
			this.renderTraceList();

			if (this.traces.length > 0) {
				this.selectTrace(0);
			}
		} catch (error) {
			console.error('Failed to load traces:', error);
			this.setStatus(`Error: ${error.message}`);
			this.elements.traceContent.textContent = `Failed to load traces: ${error.message}`;
		}
	}

	renderTraceList() {
		this.elements.listItems.innerHTML = '';

		this.traces.forEach((trace, index) => {
			const item = document.createElement('div');
			item.className = 'list-item';
			if (index === this.currentTraceIndex) {
				item.classList.add('active');
			}

			// Add fail indicator if trace has failure annotation
			if (trace.canonical_annotation && trace.canonical_annotation.label === 'fail') {
				item.classList.add('fail');
			}

			item.textContent = `${trace.trace_id.substring(0, 20)}... (${trace.scenario})`;
			item.addEventListener('click', () => this.selectTrace(index));

			this.elements.listItems.appendChild(item);
		});
	}

	selectTrace(index) {
		if (index < 0 || index >= this.traces.length) return;

		this.currentTraceIndex = index;
		const trace = this.traces[index];

		// Update display
		this.elements.currentTrace.textContent = trace.trace_id;
		this.renderTraceContent(trace);
		this.loadTraceAnnotations(trace);

		// Update list highlighting
		document.querySelectorAll('.list-item').forEach((item, i) => {
			item.classList.toggle('active', i === index);
		});

		// Update footer
		this.elements.footerProgress.textContent = `${index + 1} / ${this.traces.length}`;
	}

	renderTraceContent(trace) {
		const lines = [];
		lines.push(`Trace ID: ${trace.trace_id}`);
		lines.push(`Scenario: ${trace.scenario}`);
		lines.push(`Status: ${trace.success ? 'SUCCESS' : 'FAILED'}`);
		lines.push(`Started: ${new Date(trace.started_at).toISOString()}`);
		lines.push(`Duration: ${this.getDuration(trace)}`);
		lines.push('');

		if (trace.metrics) {
			lines.push('Metrics:');
			for (const [key, value] of Object.entries(trace.metrics)) {
				lines.push(`  ${key}: ${value}`);
			}
			lines.push('');
		}

		if (trace.metadata) {
			lines.push('Metadata:');
			lines.push(JSON.stringify(trace.metadata, null, 2));
			lines.push('');
		}

		if (trace.annotations && trace.annotations.length > 0) {
			lines.push('Existing Annotations:');
			trace.annotations.forEach((ann) => {
				lines.push(`  Annotator: ${ann.annotator_id}`);
				lines.push(`  Label: ${ann.label}`);
				if (ann.primary_failure) {
					lines.push(`  Primary Failure: ${ann.primary_failure}`);
				}
				lines.push(`  Confidence: ${(ann.confidence * 100).toFixed(0)}%`);
				if (ann.notes) {
					lines.push(`  Notes: ${ann.notes}`);
				}
			});
		}

		this.elements.traceContent.textContent = lines.join('\n');
	}

	getDuration(trace) {
		const start = new Date(trace.started_at);
		const end = new Date(trace.ended_at);
		const ms = end - start;
		return `${ms}ms`;
	}

	async loadTraceAnnotations(trace) {
		// For now, display existing annotations
		// Could enhance with similarity search or related annotations
		if (trace.annotations && trace.annotations.length > 0) {
			this.currentAnnotation = trace.canonical_annotation;
			this.populateFormFromAnnotation(this.currentAnnotation);
		} else {
			this.resetForm();
			this.currentAnnotation = null;
		}
	}

	nextTrace() {
		if (this.traces.length === 0) {
			this.setStatus('No traces loaded');
			return;
		}
		this.selectTrace(this.currentTraceIndex + 1);
	}

	previousTrace() {
		if (this.traces.length === 0) {
			this.setStatus('No traces loaded');
			return;
		}
		this.selectTrace(this.currentTraceIndex - 1);
	}

	// === Annotation Control ===

	setLabel(label) {
		this.selectedLabel = label;

		// Update button states
		this.elements.btnPass.classList.toggle('active', label === 'pass');
		this.elements.btnFail.classList.toggle('active', label === 'fail');
		this.elements.btnUncertain.classList.toggle('active', label === 'uncertain');

		// Show/hide failure mode section
		this.elements.failureSection.style.display = label === 'fail' ? 'block' : 'none';

		// Update status
		this.updateAnnotationStatus();
	}

	populateFormFromAnnotation(ann) {
		if (!ann) return;

		// Set label buttons
		this.setLabel(ann.label);

		// Set failure mode
		if (ann.primary_failure) {
			this.elements.primaryFailure.value = ann.primary_failure;
		}

		// Set secondary failures
		if (ann.secondary_failures && ann.secondary_failures.length > 0) {
			const checkboxes = this.elements.secondaryFailures.querySelectorAll('input[type="checkbox"]');
			checkboxes.forEach((cb) => {
				cb.checked = ann.secondary_failures.includes(cb.value);
			});
		}

		// Set confidence
		const confidence = Math.round(ann.confidence * 100);
		this.elements.confidenceSlider.value = confidence;
		this.elements.confidenceValue.textContent = confidence + '%';

		// Set notes
		if (ann.notes) {
			this.elements.notes.value = ann.notes;
		}

		// Set defer status
		if (ann.deferred) {
			this.elements.deferCheckbox.checked = true;
			this.elements.deferReason.value = ann.deferred_reason || '';
			this.elements.deferReason.style.display = 'block';
		}
	}

	resetForm() {
		this.selectedLabel = null;
		this.elements.btnPass.classList.remove('active');
		this.elements.btnFail.classList.remove('active');
		this.elements.btnUncertain.classList.remove('active');
		this.elements.primaryFailure.value = '';
		this.elements.secondaryFailures.querySelectorAll('input[type="checkbox"]').forEach((cb) => {
			cb.checked = false;
		});
		this.elements.confidenceSlider.value = 50;
		this.elements.confidenceValue.textContent = '50%';
		this.elements.notes.value = '';
		this.elements.deferCheckbox.checked = false;
		this.elements.deferReason.value = '';
		this.elements.deferReason.style.display = 'none';
		this.elements.failureSection.style.display = 'none';
		this.currentAnnotation = null;
	}

	toggleDefer() {
		this.elements.deferCheckbox.checked = !this.elements.deferCheckbox.checked;
		this.elements.deferReason.style.display = this.elements.deferCheckbox.checked ? 'block' : 'none';
	}

	async submitAnnotation() {
		if (!this.selectedLabel) {
			alert('Please select Pass, Fail, or Uncertain');
			return;
		}

		const trace = this.traces[this.currentTraceIndex];
		if (!trace) {
			alert('No trace selected');
			return;
		}

		const payload = {
			annotator_id: this.annotatorID,
			label: this.selectedLabel,
			primary_failure: this.elements.primaryFailure.value || undefined,
			confidence: parseInt(this.elements.confidenceSlider.value) / 100,
			notes: this.elements.notes.value || undefined,
			deferred: this.elements.deferCheckbox.checked,
			deferred_reason: this.elements.deferReason.value || undefined,
		};

		// Collect secondary failures
		const secondaryFailures = [];
		this.elements.secondaryFailures.querySelectorAll('input[type="checkbox"]:checked').forEach((cb) => {
			secondaryFailures.push(cb.value);
		});
		if (secondaryFailures.length > 0) {
			payload.secondary_failures = secondaryFailures;
		}

		try {
			this.setStatus('Submitting annotation...');
			const response = await fetch(`${this.apiBaseURL}/traces/${trace.trace_id}/feedback`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify(payload),
			});

			if (!response.ok) {
				throw new Error(`HTTP ${response.status}`);
			}

			const result = await response.json();
			this.setStatus(`Annotation saved: ${result.id}`);
			this.updateAnnotationStatus();

			// Auto-advance to next trace
			setTimeout(() => this.nextTrace(), 500);
		} catch (error) {
			console.error('Failed to submit annotation:', error);
			alert(`Error: ${error.message}`);
		}
	}

	updateAnnotationStatus() {
		if (!this.selectedLabel) {
			this.elements.annotationStatus.textContent = '';
			return;
		}

		const confidence = parseInt(this.elements.confidenceSlider.value);
		const statusText = `${this.selectedLabel.toUpperCase()} • ${confidence}% confidence`;
		this.elements.annotationStatus.textContent = statusText;
		this.elements.annotationStatus.className = `status ${this.selectedLabel}`;
	}

	// === Clustering & Rubrics ===

	async loadClusters() {
		try {
			const response = await fetch(`${this.apiBaseURL}/clusters?dimension=failure_mode`);
			if (!response.ok) return;

			const data = await response.json();
			this.renderClusters(data.clusters || {});
		} catch (error) {
			console.error('Failed to load clusters:', error);
		}
	}

	renderClusters(clusters) {
		const html = [];

		for (const [clusterName, trends] of Object.entries(clusters)) {
			trends.forEach((trend) => {
				const div = document.createElement('div');
				div.className = 'cluster-item';
				div.innerHTML = `
					<div class="cluster-item-name">${trend.failure_mode || clusterName}</div>
					<span class="cluster-item-count">${trend.count}</span>
					<span class="cluster-item-percentage">${trend.percentage.toFixed(1)}%</span>
				`;
				this.elements.clustersList.appendChild(div);
			});
		}

		if (html.length === 0) {
			this.elements.clustersList.innerHTML = '<p class="placeholder">No failure clusters yet</p>';
		}
	}

	async loadRubrics() {
		try {
			// For now, show available rubrics from the first trace
			const response = await fetch(`${this.apiBaseURL}/traces`);
			if (!response.ok) return;

			const data = await response.json();
			if (data.traces && data.traces.length > 0) {
				const trace = data.traces[0];
				if (trace.available_rubrics) {
					this.renderRubrics(trace.available_rubrics);
				}
			}
		} catch (error) {
			console.error('Failed to load rubrics:', error);
		}
	}

	renderRubrics(rubrics) {
		this.elements.rubricsList.innerHTML = '';

		for (const [rubricID, rubric] of Object.entries(rubrics)) {
			if (!rubric || !rubric.id) continue;

			const div = document.createElement('div');
			div.className = 'rubric-item';
			div.innerHTML = `
				<div class="rubric-item-id">${rubric.id}</div>
				<div class="rubric-item-name">${rubric.name || 'Unknown'}</div>
				<span class="rubric-item-dimension">${rubric.primary_dimension || 'N/A'}</span>
				<div class="rubric-item-description">${rubric.description || 'No description'}</div>
			`;
			this.elements.rubricsList.appendChild(div);
		}
	}

	switchTab(tabName) {
		// Update active tab button
		this.elements.tabBtns.forEach((btn) => {
			btn.classList.toggle('active', btn.dataset.tab === tabName);
		});

		// Update active tab content
		document.querySelectorAll('.tab-content').forEach((content) => {
			content.classList.remove('active');
		});
		const activeContent = document.getElementById(`tab-${tabName}`);
		if (activeContent) {
			activeContent.classList.add('active');
		}
	}

	// === Utility ===

	setStatus(message) {
		this.elements.footerStatus.textContent = message;
	}

	getOrCreateAnnotatorID() {
		let id = localStorage.getItem('eval-annotator-id');
		if (!id) {
			id = `annotator-${Math.random().toString(36).substring(7)}`;
			localStorage.setItem('eval-annotator-id', id);
		}
		return id;
	}
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
	window.evalReview = new EvalReviewInterface();
});
