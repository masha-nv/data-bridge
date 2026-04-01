import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { inject } from '@angular/core';
import { Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class AppService {
  isEnvSelectionEnabled = signal(false);
  intent = signal('');
  http = inject(HttpClient);
  enableEnvSelection(flag: boolean) {
    this.isEnvSelectionEnabled.update(() => flag);
  }

  triggerIntentChanged(intent: string) {
    this.intent.update(() => intent);
  }

  search(payload: {
    searchBy: string[];
    beneId: string;
    beneName: string;
    envs: string[];
  }) {
    return this.http.post('/api/search', payload);
  }

  getTables(env: string) {
    return this.http.get<string[]>(`/api/tables?env=${env}`);
  }

  moveData(payload: { table: string; fromEnv: string; toEnv: string }) {
    return this.http.post('/api/move', payload);
  }

  handleAction(action: 'move' | 'search', payload: any): Observable<any> {
    if (action === 'move') {
      return this.moveData(payload);
    } else {
      return this.search(payload);
    }
  }
}
