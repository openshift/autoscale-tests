import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { sharedStyles } from './styles/shared-styles.js';

@customElement('testgrid-grid-row-name')
export class TestgridGridRowName extends LitElement{
    static styles = [sharedStyles, css`
    :host {
      text-align: left;
      font-family: var(--font-family);
      display: inline-block;
      background-color: #ccd;
      color: #224;
      min-height: 1.2em;
      max-width: 300px;
      width:200px;
      padding: .1em .3em;
      box-sizing: border-box;
      min-height: 22px;
      max-height: 22px;
      white-space:nowrap;
      min-width: 300px;
      overflow-x: clip;
      text-overflow: ellipsis;
      position: sticky;
      left: 0;
      cursor: pointer;
    }
    /* When expanded, let the overlay escape the fixed-width column and paint
       over the cells to the right without shifting the grid. */
    :host([expanded]) {
      overflow: visible;
      z-index: 100;
    }
    /* Full, untruncated name shown on double-click. Absolutely positioned so it
       overlays instead of widening the column. user-select: all lets a single
       click select the whole name to copy. */
    .full {
      position: absolute;
      top: 0;
      left: 0;
      background: #224;
      color: #fff;
      padding: .1em .5em;
      border-radius: 3px;
      white-space: nowrap;
      user-select: all;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
      z-index: 101;
    }
  `];

    @property() name: String;

    // Reflected so the :host([expanded]) style applies. Toggled by double-click.
    @property({ type: Boolean, reflect: true }) expanded = false;

    private toggleExpanded() {
        this.expanded = !this.expanded;
    }

    render(){
        return html`<span
            title="${this.name}"
            @dblclick="${this.toggleExpanded}"
            >${this.name}</span
          >
          ${this.expanded
            ? html`<div class="full" @dblclick="${this.toggleExpanded}">
                ${this.name}
              </div>`
            : ''}`;
    }
}
