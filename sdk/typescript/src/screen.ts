import type { Cell, Position, ScreenData } from "./types.js";
export class Screen {
  readonly width;
  readonly height;
  readonly cells;
  readonly cursor;
  readonly revision;
  readonly text;
  constructor(data: ScreenData) {
    this.width = data.width;
    this.height = data.height;
    this.revision = data.revision;
    this.text = data.text;
    this.cursor = Object.freeze({ ...data.cursor });
    this.cells = Object.freeze(data.cells.map((c) => Object.freeze({ ...c })));
    Object.freeze(this);
  }
  contains(text: string): boolean {
    return this.text.includes(text);
  }
  line(row: number): string {
    if (row < 0 || row >= this.height) return "";
    let s = "";
    for (let c = 0; c < this.width; c++) {
      const x = this.cellAt(c, row);
      if (x?.width) s += x.text;
    }
    return s;
  }
  cellAt(column: number, row: number): Cell | undefined {
    if (column < 0 || row < 0 || column >= this.width || row >= this.height)
      return;
    return this.cells[row * this.width + column];
  }
  find(text: string): Position | undefined {
    if (text === "" && this.width > 0 && this.height > 0)
      return { column: 0, row: 0 };
    for (let r = 0; r < this.height; r++)
      for (let c = 0; c < this.width; c++) {
        if (this.cellAt(c, r)?.width === 0) continue;
        let value = "";
        for (let x = c; x < this.width; x++) {
          const cell = this.cellAt(x, r);
          if (cell?.width) value += cell.text;
          if (value.startsWith(text)) return { column: c, row: r };
          if (!text.startsWith(value)) break;
        }
      }
    return;
  }
}
