#!/usr/bin/env python3
"""
Genie UI Accessibility Verification Script
Validates Phase 1 ARIA improvements, semantic roles, and CSS accessibility features
"""

import re
import sys
import json
from pathlib import Path
from collections import defaultdict


class A11yVerifier:
    def __init__(self, html_path: str, css_path: str):
        self.html_path = Path(html_path)
        self.css_path = Path(css_path)
        self.html_content = self._load_file(html_path)
        self.css_content = self._load_file(css_path)

        # Tracking metrics
        self.aria_labels = []
        self.aria_describedby = []
        self.aria_live = []
        self.aria_selected = []
        self.aria_invalid = []
        self.aria_busy = []
        self.aria_hidden = []
        self.tabindex_values = defaultdict(list)
        self.semantic_roles = defaultdict(list)
        self.focus_visible_rules = []
        self.css_features = {
            'focus_visible': False,
            'skeleton_animation': False,
            'aria_invalid_styling': False,
            'aria_busy_styling': False,
        }

    def _load_file(self, path: str) -> str:
        """Load file content safely"""
        try:
            return Path(path).read_text(encoding='utf-8')
        except FileNotFoundError:
            print(f"ERROR: File not found: {path}")
            sys.exit(1)

    def verify_aria_labels(self):
        """Count and locate aria-label attributes"""
        pattern = r'aria-label="([^"]*)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            self.aria_labels.append({
                'value': match.group(1),
                'position': match.start()
            })
        return len(self.aria_labels)

    def verify_aria_describedby(self):
        """Count aria-describedby attributes"""
        pattern = r'aria-describedby="([^"]*)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            ids = match.group(1).split()
            for id_ref in ids:
                self.aria_describedby.append(id_ref)
        return len(self.aria_describedby)

    def verify_aria_live(self):
        """Count aria-live regions"""
        pattern = r'aria-live="(polite|assertive|off)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            self.aria_live.append({
                'type': match.group(1),
                'position': match.start()
            })
        return len(self.aria_live)

    def verify_aria_selected(self):
        """Count aria-selected on tabs"""
        pattern = r'aria-selected="(true|false)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            self.aria_selected.append(match.group(1))
        return len(self.aria_selected)

    def verify_semantic_roles(self):
        """Count semantic roles"""
        pattern = r'role="([^"]*)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            role = match.group(1)
            self.semantic_roles[role].append(match.start())
        return len(self.semantic_roles)

    def verify_tabindex(self):
        """Verify tabindex management"""
        pattern = r'tabindex="(-?\d+)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            value = int(match.group(1))
            self.tabindex_values[value].append(match.start())
        return self.tabindex_values

    def verify_aria_invalid(self):
        """Count aria-invalid attributes"""
        pattern = r'aria-invalid="(true|false)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            self.aria_invalid.append(match.group(1))
        return len(self.aria_invalid)

    def verify_aria_busy(self):
        """Count aria-busy attributes"""
        pattern = r'aria-busy="(true|false)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            self.aria_busy.append(match.group(1))
        return len(self.aria_busy)

    def verify_aria_hidden(self):
        """Count aria-hidden attributes"""
        pattern = r'aria-hidden="(true|false)"'
        matches = re.finditer(pattern, self.html_content)
        for match in matches:
            self.aria_hidden.append(match.group(1))
        return len(self.aria_hidden)

    def verify_css_focus_visible(self):
        """Check for focus-visible CSS rules"""
        pattern = r'(.*?):focus-visible\s*{([^}]+)}'
        matches = re.finditer(pattern, self.css_content)
        for match in matches:
            selector = match.group(1).strip()
            declarations = match.group(2)
            # Check for outline
            if 'outline' in declarations and '2px' in declarations:
                self.focus_visible_rules.append({
                    'selector': selector,
                    'has_outline': True
                })
                self.css_features['focus_visible'] = True
        return self.css_features['focus_visible']

    def verify_css_skeleton_animation(self):
        """Check for skeleton loading animation"""
        pattern = r'@keyframes\s+skeleton-load'
        if re.search(pattern, self.css_content):
            self.css_features['skeleton_animation'] = True
            # Verify it's applied to .skeleton
            if 'animation: skeleton-load' in self.css_content:
                return True
        return False

    def verify_css_aria_invalid_styling(self):
        """Check for aria-invalid error state styling"""
        pattern = r'\[aria-invalid="true"\]'
        if re.search(pattern, self.css_content):
            # Check for color styling (--bad color)
            if '--bad' in self.css_content:
                self.css_features['aria_invalid_styling'] = True
                return True
        return False

    def verify_css_aria_busy_styling(self):
        """Check for aria-busy button loading state"""
        pattern = r'\[aria-busy="true"\]'
        if re.search(pattern, self.css_content):
            # Check for opacity and cursor changes
            if 'cursor: not-allowed' in self.css_content:
                self.css_features['aria_busy_styling'] = True
                return True
        return False

    def verify_all(self):
        """Run all verification checks"""
        self.verify_aria_labels()
        self.verify_aria_describedby()
        self.verify_aria_live()
        self.verify_aria_selected()
        self.verify_semantic_roles()
        self.verify_tabindex()
        self.verify_aria_invalid()
        self.verify_aria_busy()
        self.verify_aria_hidden()
        self.verify_css_focus_visible()
        self.verify_css_skeleton_animation()
        self.verify_css_aria_invalid_styling()
        self.verify_css_aria_busy_styling()

    def generate_report(self) -> dict:
        """Generate accessibility report"""
        self.verify_all()

        # Calculate scores
        aria_label_score = len(self.aria_labels)
        aria_describedby_score = len(self.aria_describedby)
        aria_live_score = len(self.aria_live)
        aria_selected_score = len(self.aria_selected)
        semantic_roles_count = len(self.semantic_roles)

        # Expected targets based on CLAUDE.md requirements
        expected_aria_labels = 50
        expected_semantic_roles = 10

        # Tabindex verification
        has_correct_tabindex = (
            0 in self.tabindex_values and
            -1 in self.tabindex_values
        )

        # CSS features verification
        all_css_features = all(self.css_features.values())

        # Calculate accessibility score (0-10)
        score_components = {
            'aria_labels': min(aria_label_score / expected_aria_labels, 1.0),
            'aria_describedby': 1.0 if aria_describedby_score > 0 else 0.0,
            'aria_live': 1.0 if aria_live_score >= 5 else 0.5,
            'aria_selected': 1.0 if aria_selected_score >= 4 else 0.5,
            'semantic_roles': min(semantic_roles_count / expected_semantic_roles, 1.0),
            'tabindex': 1.0 if has_correct_tabindex else 0.0,
            'css_focus_visible': 1.0 if self.css_features['focus_visible'] else 0.0,
            'css_skeleton': 1.0 if self.css_features['skeleton_animation'] else 0.0,
            'css_aria_invalid': 1.0 if self.css_features['aria_invalid_styling'] else 0.0,
            'css_aria_busy': 1.0 if self.css_features['aria_busy_styling'] else 0.0,
        }

        accessibility_score = sum(score_components.values()) / len(score_components) * 10

        report = {
            'timestamp': __import__('datetime').datetime.now().isoformat(),
            'html_file': str(self.html_path),
            'css_file': str(self.css_path),

            'scorecard': {
                'aria_labels': {
                    'found': aria_label_score,
                    'target': expected_aria_labels,
                    'status': 'PASS' if aria_label_score >= expected_aria_labels else 'PARTIAL',
                    'percentage': min((aria_label_score / expected_aria_labels) * 100, 100)
                },
                'aria_describedby': {
                    'found': aria_describedby_score,
                    'target': 5,
                    'status': 'PASS' if aria_describedby_score >= 5 else 'PARTIAL'
                },
                'aria_live': {
                    'found': aria_live_score,
                    'target': 5,
                    'status': 'PASS' if aria_live_score >= 5 else 'PARTIAL'
                },
                'aria_selected': {
                    'found': aria_selected_score,
                    'target': 4,
                    'status': 'PASS' if aria_selected_score == 4 else 'PARTIAL'
                },
                'semantic_roles': {
                    'found': semantic_roles_count,
                    'target': expected_semantic_roles,
                    'roles': dict(self.semantic_roles),
                    'status': 'PASS' if semantic_roles_count >= expected_semantic_roles else 'PARTIAL'
                },
                'tabindex_management': {
                    'status': 'PASS' if has_correct_tabindex else 'FAIL',
                    'active_found': 0 in self.tabindex_values,
                    'inactive_found': -1 in self.tabindex_values,
                    'active_count': len(self.tabindex_values.get(0, [])),
                    'inactive_count': len(self.tabindex_values.get(-1, []))
                }
            },

            'css_features': {
                'focus_visible': {
                    'status': 'PASS' if self.css_features['focus_visible'] else 'FAIL',
                    'description': 'CSS :focus-visible rules with 2px outline',
                    'found': self.css_features['focus_visible']
                },
                'skeleton_animation': {
                    'status': 'PASS' if self.css_features['skeleton_animation'] else 'FAIL',
                    'description': '@keyframes skeleton-load animation',
                    'found': self.css_features['skeleton_animation']
                },
                'aria_invalid_styling': {
                    'status': 'PASS' if self.css_features['aria_invalid_styling'] else 'FAIL',
                    'description': '[aria-invalid="true"] error state styling',
                    'found': self.css_features['aria_invalid_styling']
                },
                'aria_busy_styling': {
                    'status': 'PASS' if self.css_features['aria_busy_styling'] else 'FAIL',
                    'description': '[aria-busy="true"] button loading state',
                    'found': self.css_features['aria_busy_styling']
                }
            },

            'phase_1_improvements': {
                'aria_labels': 'PASS' if aria_label_score >= expected_aria_labels else 'PARTIAL',
                'semantic_roles': 'PASS' if semantic_roles_count >= expected_semantic_roles else 'PARTIAL',
                'tabindex_management': 'PASS' if has_correct_tabindex else 'FAIL',
                'aria_live_regions': 'PASS' if aria_live_score >= 5 else 'PARTIAL',
                'aria_selected_tabs': 'PASS' if aria_selected_score == 4 else 'PARTIAL',
                'aria_describedby': 'PASS' if aria_describedby_score > 0 else 'FAIL'
            },

            'accessibility_score': round(accessibility_score, 1),
            'score_components': {k: round(v * 10, 1) for k, v in score_components.items()},

            'details': {
                'aria_labels_sample': self.aria_labels[:5],
                'semantic_roles_distribution': {role: len(positions) for role, positions in self.semantic_roles.items()},
                'tabindex_distribution': {str(k): len(v) for k, v in self.tabindex_values.items()},
                'aria_live_types': {item['type']: 1 for item in self.aria_live},
                'css_features_all_present': all_css_features
            }
        }

        return report


def print_report(report: dict):
    """Pretty print the accessibility report"""
    print("\n" + "=" * 80)
    print("GENIE UI ACCESSIBILITY VERIFICATION REPORT")
    print("=" * 80)
    print(f"\nTimestamp: {report['timestamp']}")
    print(f"HTML File: {report['html_file']}")
    print(f"CSS File: {report['css_file']}")

    print("\n" + "-" * 80)
    print("SCORECARD: ARIA ATTRIBUTES")
    print("-" * 80)

    for metric, data in report['scorecard'].items():
        if metric == 'semantic_roles':
            status = data['status']
            found = data['found']
            target = data['target']
            percentage = (found / target * 100) if target > 0 else 0
            print(f"\n{metric.upper().replace('_', ' ')}")
            print(f"  Status: {status}")
            print(f"  Found: {found}/{target} ({percentage:.1f}%)")
            print(f"  Roles: {', '.join(f'{role}({count})' for role, count in data['roles'].items())}")
        elif metric == 'tabindex_management':
            print(f"\n{metric.upper().replace('_', ' ')}")
            print(f"  Status: {data['status']}")
            print(f"  Active (tabindex=0): {data['active_count']} found ✓" if data['active_found'] else f"  Active (tabindex=0): NOT FOUND ✗")
            print(f"  Inactive (tabindex=-1): {data['inactive_count']} found ✓" if data['inactive_found'] else f"  Inactive (tabindex=-1): NOT FOUND ✗")
        else:
            status = data['status']
            found = data.get('found', 'N/A')
            target = data.get('target', 'N/A')
            pct = f" ({data.get('percentage', 0):.1f}%)" if 'percentage' in data else ""
            print(f"\n{metric.upper().replace('_', ' ')}")
            print(f"  Status: {status}")
            print(f"  Found: {found}/{target}{pct}")

    print("\n" + "-" * 80)
    print("CSS ACCESSIBILITY FEATURES")
    print("-" * 80)

    for feature, data in report['css_features'].items():
        status = "✓ PASS" if data['status'] == 'PASS' else "✗ FAIL"
        print(f"\n{feature.upper().replace('_', ' ')}")
        print(f"  {status}")
        print(f"  {data['description']}")

    print("\n" + "-" * 80)
    print("PHASE 1 IMPROVEMENTS SUMMARY")
    print("-" * 80)

    for improvement, status in report['phase_1_improvements'].items():
        symbol = "✓" if status == "PASS" else "◐" if status == "PARTIAL" else "✗"
        print(f"{symbol} {improvement.replace('_', ' ').title()}: {status}")

    print("\n" + "-" * 80)
    print("ACCESSIBILITY SCORE")
    print("-" * 80)

    score = report['accessibility_score']
    if score >= 8:
        rating = "EXCELLENT"
    elif score >= 6:
        rating = "GOOD"
    elif score >= 4:
        rating = "FAIR"
    else:
        rating = "POOR"

    print(f"\nOverall Score: {score}/10 ({rating})")
    print("\nComponent Scores:")
    for component, score_val in report['score_components'].items():
        bar_length = int(score_val / 10 * 20)
        bar = "█" * bar_length + "░" * (20 - bar_length)
        print(f"  {component.replace('_', ' ').title():30s} {bar} {score_val:.1f}/10")

    print("\n" + "-" * 80)
    print("RECOMMENDATIONS")
    print("-" * 80)

    missing = []
    if report['scorecard']['aria_labels']['found'] < report['scorecard']['aria_labels']['target']:
        missing.append(f"Add more aria-labels (need {report['scorecard']['aria_labels']['target'] - report['scorecard']['aria_labels']['found']} more)")

    if not report['css_features']['focus_visible']['found']:
        missing.append("Add :focus-visible CSS rules with 2px outline")

    if not report['css_features']['skeleton_animation']['found']:
        missing.append("Add @keyframes skeleton-load animation for loading states")

    if not report['css_features']['aria_invalid_styling']['found']:
        missing.append("Add [aria-invalid] CSS styling for error states")

    if not report['css_features']['aria_busy_styling']['found']:
        missing.append("Add [aria-busy] CSS styling for loading buttons")

    if report['scorecard']['tabindex_management']['status'] != 'PASS':
        missing.append("Implement tabindex=0 for active tab, tabindex=-1 for inactive tabs")

    if missing:
        for i, rec in enumerate(missing, 1):
            print(f"{i}. {rec}")
    else:
        print("✓ All accessibility improvements are in place!")

    print("\n" + "=" * 80)


def output_json(report: dict, filepath: str = None):
    """Output report as JSON"""
    if filepath:
        with open(filepath, 'w') as f:
            json.dump(report, f, indent=2)
        print(f"\nJSON report written to: {filepath}")
    else:
        print("\n" + json.dumps(report, indent=2))


if __name__ == '__main__':
    # Use relative paths based on script location
    script_dir = os.path.dirname(os.path.abspath(__file__))
    html_file = os.path.join(script_dir, 'index.html')
    css_file = os.path.join(script_dir, 'styles.css')

    verifier = A11yVerifier(html_file, css_file)
    report = verifier.generate_report()

    print_report(report)

    # Optional: save JSON report
    json_output_path = os.path.join(script_dir, 'a11y-report.json')
    output_json(report, json_output_path)

    # Exit with appropriate code
    sys.exit(0 if report['accessibility_score'] >= 8 else 1)
