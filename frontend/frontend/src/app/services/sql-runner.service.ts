import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { EnvironmentService } from './environment.service';

export interface SqlStatementResult {
  statementNumber: number;
  statement: string;
  columns: string[];
  rows: string[][];
  rowCount: number;
  truncated?: boolean;
  error: string;
}

export interface SqlRunnerResponse {
  environment: string;
  results: SqlStatementResult[];
}

@Injectable({ providedIn: 'root' })
export class SqlRunnerService {
  private readonly httpClient = inject(HttpClient);
  private readonly environmentService = inject(EnvironmentService);

  run(environment: string, sql: string): Observable<SqlRunnerResponse> {
    return this.httpClient.post<SqlRunnerResponse>(
      this.environmentService.getApiUrl('/api/marx/sql/run'),
      {
        environment,
        sql,
      },
    );
  }
}