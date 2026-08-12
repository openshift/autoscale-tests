// testgrid-grid-cell.ts
import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { sharedStyles } from './styles/shared-styles.js';

@customElement('testgrid-grid-cell')
export class TestgridGridCell extends LitElement {
  // Styling for status attribute corresponds to test_status.proto enum.
  static styles = [sharedStyles, css`
    :host {
      min-width: 80px;
      width: 80px;
      min-height: 22px;
      max-height: 22px;
      color: #000;
      background-color: #ccc;
      text-align: center;
      font-family: var(--font-family);
      font-weight: bold;
      display: flex;
      justify-content: center;
      align-content: center;
      flex-direction: column;
      box-sizing: border-box;
      font-size: var(--font-size-xs);
    }

    :host([status='NO_RESULT']) {
      background-color: transparent;
    }

    :host([status='PASS']),
    :host([status='PASS_WITH_ERRORS']) {
      background-color: #4d7;
      color: #273;
    }

    :host([status='PASS_WITH_SKIPS']),
    :host([status='BUILD_PASSED']) {
      background-color: #bfd;
      color: #465;
    }

    :host([status='RUNNING']),
    :host([status='CATEGORIZED_ABORT']),
    :host([status='UNKNOWN']),
    :host([status='CANCEL']),
    :host([status='BLOCKED']) {
      background-color: #ccd;
      color: #446;
    }

    :host([status='TIMED_OUT']),
    :host([status='CATEGORIZED_FAIL']),
    :host([status='FAIL']),
    :host([status='FLAKY']) {
      background-color: #a24;
      color: #fdd;
    }

    :host([status='BUILD_FAIL']) {
      background-color: #111;
      color: #ddd;
    }

    :host([status='FLAKY']) {
      background-color: #63a;
      color: #dcf;
    }

    a {
      text-decoration: inherit;
      color: inherit;
      width: 100%;
      height: 100%;
    }
  `];

  @property({ reflect: true, attribute: 'status' }) status: string;
  @property() icon: string;
  @property() rowName: string = '';
  @property() buildId: string = '';
  @property() dashboardName: string = '';
  @property() tabName: string = '';

  // GCS prefix for the backing test group (e.g.
  // "test-platform-results/logs/<job>"), supplied by the grid from the tab
  // config. Combined with buildId to link a cell to its real build result.
  @property() gcsPrefix: string = '';

  // Link to the actual build result for this cell on prow. Empty when we lack a
  // build ID or the tab's GCS prefix (e.g. placeholder/no-result cells), in
  // which case the cell renders without a link.
  private generateTestUrl(): string {
    if (!this.buildId || !this.gcsPrefix) {
      return '';
    }
    return `https://prow.ci.openshift.org/view/gs/${this.gcsPrefix}/${this.buildId}`;
  }

  render() {
    const testUrl = this.generateTestUrl();
    return testUrl
      ? html`<a href="${testUrl}" target="_blank" rel="noopener noreferrer"
          ><span>${this.icon}</span></a
        >`
      : html`<span>${this.icon}</span>`;
  }
}
