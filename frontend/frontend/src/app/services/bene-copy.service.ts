import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { EnvironmentService } from './environment.service';

export type BeneCopyJobStatus = 'queued' | 'running' | 'completed' | 'failed';

export interface BeneCopyJobSubmissionResponse {
  jobId: string;
  engine: string;
  status: BeneCopyJobStatus;
  submittedAt: string;
  sourceEnvironment: string;
  targetEnvironment: string;
  beneLinkPartKey?: string;
  beneLinkKey: string;
  message: string;
}

export interface BeneCopyJobStatusResponse {
  jobId: string;
  engine: string;
  status: BeneCopyJobStatus;
  submittedAt: string;
  updatedAt: string;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
  sourceEnvironment: string;
  targetEnvironment: string;
  beneLinkPartKey?: string;
  beneLinkKey: string;
  currentTable?: string;
  copiedRows: number;
  skippedRows: number;
  message?: string;
  error?: string;
}

export interface BeneCopyJobRequest {
  sourceEnvironment: string;
  targetEnvironment: string;
  beneLinkKey: string;
}

export interface BeneCopyMovedataJobRequest extends BeneCopyJobRequest {
  beneLinkPartKey: string;
  engine?: 'marx-movedata';
}

export interface BeneCopyTableHistoryEntry {
  tableName: string;
  status: 'running' | 'completed' | 'failed';
  startedAt: string;
  completedAt?: string;
  durationMs?: number;
  copiedRows: number;
  skippedRows: number;
  error?: string;
}

export interface BeneCopyHistoryListResponse {
  jobs: BeneCopyJobStatusResponse[];
}

export interface BeneCopyHistoryDetailResponse {
  job: BeneCopyJobStatusResponse;
  tables: BeneCopyTableHistoryEntry[];
}

@Injectable({ providedIn: 'root' })
export class BeneCopyService {
  private readonly httpClient = inject(HttpClient);
  private readonly environmentService = inject(EnvironmentService);

  submitMovedataJob(request: BeneCopyMovedataJobRequest): Observable<BeneCopyJobSubmissionResponse> {
    return this.httpClient.post<BeneCopyJobSubmissionResponse>(
      this.environmentService.getApiUrl('/api/marx/beneficiaries/copy'),
      {
        ...request,
        engine: 'marx-movedata',
      },
    );
  }

  getJobStatus(jobId: string): Observable<BeneCopyJobStatusResponse> {
    return this.httpClient.get<BeneCopyJobStatusResponse>(
      this.environmentService.getApiUrl(`/api/marx/beneficiaries/copy/status?jobId=${encodeURIComponent(jobId)}`),
    );
  }

  getHistory(limit = 20, engine = 'marx-movedata'): Observable<BeneCopyHistoryListResponse> {
    return this.httpClient.get<BeneCopyHistoryListResponse>(
      this.environmentService.getApiUrl(`/api/marx/beneficiaries/copy/history?limit=${encodeURIComponent(String(limit))}&engine=${encodeURIComponent(engine)}`),
    );
  }

  getJobHistory(jobId: string): Observable<BeneCopyHistoryDetailResponse> {
    return this.httpClient.get<BeneCopyHistoryDetailResponse>(
      this.environmentService.getApiUrl(`/api/marx/beneficiaries/copy/history/${encodeURIComponent(jobId)}`),
    );
  }
}