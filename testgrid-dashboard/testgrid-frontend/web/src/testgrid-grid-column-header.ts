import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { sharedStyles } from './styles/shared-styles.js';

@customElement('testgrid-grid-column-header')
export class TestgridGridColumnHeader extends LitElement {
  static styles = [sharedStyles, css`
    :host {
      text-align: center;
      font-family: var(--font-family);
      display: inline-block;
      background-color: #ccd;
      color: #224;
      min-height: 22px;
      max-height: 22px;
      padding: 0.1em 0.3em;
      box-sizing: border-box;
      white-space: nowrap;
      overflow-x: hidden;
      text-overflow: ellipsis;
      position: relative;
      cursor: pointer;
    }
    /* When expanded, let the overlay escape the fixed-width column and paint
       over neighbors without shifting the grid. */
    :host([expanded]) {
      overflow: visible;
      z-index: 100;
    }
    span {
      position: sticky;
      left: 0;
      right: 0;
      width: fit-content;
      margin: auto;
    }
    /* Full, untruncated value shown on double-click. Absolutely positioned so it
       overlays instead of widening the column (which would misalign the cells
       below). user-select: all lets a single click select the whole value to
       copy. */
    .full {
      position: absolute;
      top: 0;
      left: 50%;
      transform: translateX(-50%);
      background: #224;
      color: #fff;
      padding: 0.1em 0.5em;
      border-radius: 3px;
      white-space: nowrap;
      user-select: all;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
      z-index: 101;
    }
  `];

  @property() value: string;

  // Reflected so the :host([expanded]) style applies. Toggled by double-click.
  @property({ type: Boolean, reflect: true }) expanded = false;

  private toggleExpanded() {
    this.expanded = !this.expanded;
  }

  render() {
    return html`<span
        title="${this.value}"
        @dblclick="${this.toggleExpanded}"
        >${this.value}</span
      >
      ${this.expanded
      ? html`<div class="full" @dblclick="${this.toggleExpanded}">
          ${this.value}
        </div>`
      : ''}`;
  }
}
