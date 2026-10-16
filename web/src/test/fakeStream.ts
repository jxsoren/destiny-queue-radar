import type { StreamLike } from '../live/stream';

/** A scriptable EventSource. Every instance is recorded in FakeStream.all. */
export class FakeStream implements StreamLike {
  static all: FakeStream[] = [];
  static reset(): void {
    FakeStream.all = [];
  }
  static latest(): FakeStream {
    const s = FakeStream.all.at(-1);
    if (!s) throw new Error('no stream opened');
    return s;
  }

  readonly url: string;
  onerror: ((e: Event) => void) | null = null;
  readyState = 1;
  closed = false;
  private listeners = new Map<string, ((e: MessageEvent<string>) => void)[]>();

  constructor(url: string) {
    this.url = url;
    FakeStream.all.push(this);
  }

  addEventListener(type: string, listener: (e: MessageEvent<string>) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  close(): void {
    this.closed = true;
    this.readyState = 2;
  }

  emit(type: string, data: unknown): void {
    for (const fn of this.listeners.get(type) ?? []) fn(new MessageEvent(type, { data: JSON.stringify(data) }));
  }

  /** Fires an error; readyState 0 means the browser is reconnecting, 2 that it gave up. */
  fail(readyState: 0 | 2 = 2): void {
    this.readyState = readyState;
    this.onerror?.(new Event('error'));
  }
}

export const fakeStreamFactory = (url: string) => new FakeStream(url);
