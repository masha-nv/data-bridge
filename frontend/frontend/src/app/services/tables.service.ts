import { inject, Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { ENVIRONMENT } from '../enums';
import { EnvironmentService } from './environment.service';
import { MoveResponse, SearchRequest } from './interfaces/search-request';
import { Observable, tap } from 'rxjs';
@Injectable({ providedIn: 'root' })
export class TablesService {
  http = inject(HttpClient);
  searchResults = signal<Record<ENVIRONMENT, []> | null>(null);
  selectedTable = signal<string>('');
  columns = signal<string[]>([]);
  searchBy = signal<string[]>([]);

  get(env: ENVIRONMENT, table: string) {
    return this.http.get(`/api/rows?env=${env}&table=${table}`);
  }

  handleTableChange(table: string) {
    this.searchBy.set([]);
    this.getColumns(table);
  }

  getColumns(table: string) {
    return this.http
      .get<string[]>(`/api/columls?table=${table}`)
      .pipe(tap((res) => this.columns.set(res)))
      .subscribe();
  }

  getTables(env: string) {
    return this.http.get<string[]>(`/api/tables?env=${env}`);
  }

  search(payload: SearchRequest) {
    return this.http.post<Record<ENVIRONMENT, []>>('/api/search', payload).pipe(
      tap((res) => {
        this.searchResults.set(res);
      }),
    );
  }

  moveData(payload: {
    table: string;
    fromEnv: string;
    toEnv: string;
  }): Observable<MoveResponse> {
    return this.http.post<MoveResponse>('/api/move', payload);
  }
}
