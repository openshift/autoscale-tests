import { LitElement, html, PropertyValues, css } from 'lit';
import { map } from 'lit/directives/map.js';
import { customElement, property, state } from 'lit/decorators.js';
import {
  ListHeadersResponse,
  ListRowsResponse,
  // eslint-disable-next-line camelcase
  ListRowsResponse_Row,
} from './gen/pb/api/v1/data.js';
import { APIController } from './controllers/api-controller.js';
import { apiClient } from './APIClient.js';
import './testgrid-grid-headers-block.js';
import './testgrid-grid-row.js';

/**
 * Test-status codes (see test_status.proto) that mark a cell as "not a clean
 * pass" — i.e. failed, flaky, timed out, blocked, etc. A row is considered
 * interesting when any of its cells has one of these statuses. Passing/empty
 * codes (NO_RESULT=0, PASS=1, PASS_WITH_ERRORS=2, PASS_WITH_SKIPS=3, RUNNING=4,
 * BUILD_PASSED=15) are intentionally excluded.
 */
const FAILURE_STATUSES = new Set<number>([
  5, // CATEGORIZED_ABORT
  6, // UNKNOWN
  7, // CANCEL
  8, // BLOCKED
  9, // TIMED_OUT
  10, // CATEGORIZED_FAIL
  11, // BUILD_FAIL
  12, // FAIL
  13, // FLAKY
  14, // TOOL_FAIL
]);

/**
 * Class definition for `testgrid-grid` component.
 * Renders the test results grid.
 */
@customElement('testgrid-grid')
export class TestgridGrid extends LitElement {
  static styles = css`
    :host {
      display: block;
      overflow: scroll;
      width: 100%;
      height: 100%;
    }

    .grid-filter {
      position: sticky;
      left: 0;
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 4px 8px;
      font-family: var(--font-family);
      font-size: var(--font-size-xs, 12px);
    }

    .grid-filter .count {
      color: #666;
    }
  `;

  @property({ type: String, reflect: true })
  dashboardName: string = '';

  @property({ type: String, reflect: true })
  tabName: string = '';

  @state()
    // eslint-disable-next-line camelcase
  tabGridRows: Array<ListRowsResponse_Row> = [];

  @state()
  tabGridHeaders: ListHeadersResponse;

  // When true, hide test rows that always pass (or have no result), keeping only
  // rows with at least one failed/flaky/etc. cell. Defaults on because a full
  // e2e tab can hold thousands of always-passing rows, which is expensive to
  // render.
  @state()
  showOnlyFailures: boolean = true;

  // GCS prefix for this tab's backing test group (e.g.
  // "test-platform-results/logs/<job>"). Fetched from the tab config and passed
  // down to each cell so its link points at the real build result.
  @state()
  gcsPrefix: string = '';

  private headersController = new APIController<ListHeadersResponse>(this);

  private rowsController = new APIController<ListRowsResponse>(this);

  private gcsPrefixController = new APIController<string>(this);

  // Memoized "view" of the grid: headers/rows already projected onto the visible
  // columns and filtered by showOnlyFailures. Recomputed only when the source
  // data or the toggle changes (see willUpdate), NOT on every render — otherwise
  // each reactive update would re-scan every cell (tens of thousands) before Lit
  // could lay out the DOM.
  // eslint-disable-next-line camelcase
  private viewRows: Array<ListRowsResponse_Row> = [];

  private viewHeaders?: ListHeadersResponse;

  private viewBuildIds: string[] = [];

  /**
   * Lit-element lifecycle method.
   * Invoked when element properties are changed.
   */
  willUpdate(changedProperties: PropertyValues<this>) {
    if (changedProperties.has('tabName')) {
      this.fetchTabGrid();
    }
    // Only rebuild the projected/filtered view when its inputs actually change.
    if (
      changedProperties.has('tabGridRows') ||
      changedProperties.has('tabGridHeaders') ||
      changedProperties.has('showOnlyFailures')
    ) {
      this.recomputeView();
    }
  }

  /**
   * Rebuild the memoized view (viewHeaders/viewRows/viewBuildIds) from the raw
   * grid data. Projects headers and each row's cells onto the visible columns
   * (dropping undated placeholder columns), then applies the failed/flaky row
   * filter. Called from willUpdate, so render() stays cheap.
   */
  private recomputeView() {
    const colIndices = this.visibleColumnIndices;

    const filteredHeaders =
      colIndices && this.tabGridHeaders
        ? {
          ...this.tabGridHeaders,
          headers: colIndices.map(i => this.tabGridHeaders.headers[i]),
        }
        : this.tabGridHeaders;
    this.viewHeaders = filteredHeaders;
    this.viewBuildIds = filteredHeaders?.headers?.map(h => h.build) || [];

    const projectedRows: Array<ListRowsResponse_Row> = this.tabGridRows.map(
      row =>
        colIndices
          ? { ...row, cells: colIndices.map(i => row.cells[i]) }
          : row
    );
    this.viewRows = this.showOnlyFailures
      ? projectedRows.filter(TestgridGrid.rowHasFailure)
      : projectedRows;
  }

  /**
   * Lit-element lifecycle method.
   * Invoked on each update to perform rendering tasks.
   */
  /**
   * True when a row has at least one cell in a failing/flaky/etc. state.
   * The "Overall" summary row is always kept as an anchor.
   */
  private static rowHasFailure(row: ListRowsResponse_Row): boolean {
    if (row.name === 'Overall') {
      return true;
    }
    return row.cells.some(cell => FAILURE_STATUSES.has(cell?.result));
  }

  /**
   * Indices of columns worth showing: those with a real start time. Builds
   * older than the test group's `days_of_results` window are kept by the updater
   * as near-empty, undated (epoch-0) placeholder columns; we drop them so the
   * grid shows only real, dated builds. Returns null when headers haven't loaded
   * yet, meaning "keep every column" (avoids blanking cells before the header
   * fetch resolves).
   */
  private get visibleColumnIndices(): number[] | null {
    const headers = this.tabGridHeaders?.headers;
    if (!headers || headers.length === 0) {
      return null;
    }
    const indices: number[] = [];
    headers.forEach((h, i) => {
      const seconds = h.started?.seconds ? Number(h.started.seconds) : 0;
      if (seconds > 0) {
        indices.push(i);
      }
    });
    return indices;
  }

  private toggleShowOnlyFailures(e: Event) {
    this.showOnlyFailures = (e.target as HTMLInputElement).checked;
  }

  render() {
    // The heavy projection/filter work happens in recomputeView (willUpdate);
    // render just reads the memoized view so re-renders stay cheap.
    const filteredHeaders = this.viewHeaders;
    const buildIds = this.viewBuildIds;
    const rows = this.viewRows;

    return html`
      <div class="grid-filter">
        <label>
          <input
            type="checkbox"
            .checked="${this.showOnlyFailures}"
            @change="${this.toggleShowOnlyFailures}"
          />
          Show only failed/flaky tests
        </label>
        <span class="count"
          >(${rows.length} of ${this.tabGridRows.length} rows,
          ${buildIds.length} columns)</span
        >
      </div>
      <testgrid-grid-headers-block
        .headers="${filteredHeaders}"
      ></testgrid-grid-headers-block>
      ${map(
      rows,
      (row: ListRowsResponse_Row) =>
        html`<testgrid-grid-row
            .rowData="${row}"
            .buildIds="${buildIds}"
            .dashboardName="${this.dashboardName}"
            .tabName="${this.tabName}"
            .gcsPrefix="${this.gcsPrefix}"
          ></testgrid-grid-row>`
    )}
    `;
  }

  private async fetchTabGrid() {
    this.fetchTabGridRows();
    this.fetchTabGridHeaders();
    this.fetchGcsPrefix();
  }

  private async fetchGcsPrefix() {
    // Capture the tab we're fetching for; if the user switches tabs before this
    // resolves, we must not clobber the new tab's prefix with this stale result.
    const dashboardName = this.dashboardName;
    const tabName = this.tabName;
    this.gcsPrefix = '';
    try {
      const prefix = await this.gcsPrefixController.fetch(
        `tab-gcsprefix-${dashboardName}-${tabName}`,
        () => apiClient.getTabGcsPrefix(dashboardName, tabName)
      );
      if (this.dashboardName === dashboardName && this.tabName === tabName) {
        this.gcsPrefix = prefix;
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error(`Could not get tab GCS prefix: ${error}`);
    }
  }

  private async fetchTabGridRows() {
    this.tabGridRows = [];
    try {
      const data = await this.rowsController.fetch(
        `tab-rows-${this.dashboardName}-${this.tabName}`,
        () => apiClient.getTabRows(this.dashboardName, this.tabName)
      );
      // eslint-disable-next-line camelcase
      const rows: Array<ListRowsResponse_Row> = [];
      data.rows.forEach(row => rows.push(row));
      this.tabGridRows = rows;
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error(`Could not get grid rows: ${error}`);
    }
  }

  private async fetchTabGridHeaders() {
    try {
      const data = await this.headersController.fetch(
        `tab-headers-${this.dashboardName}-${this.tabName}`,
        () => apiClient.getTabHeaders(this.dashboardName, this.tabName)
      );
      this.tabGridHeaders = data;
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error(`Could not get grid headers: ${error}`);
    }
  }
}
