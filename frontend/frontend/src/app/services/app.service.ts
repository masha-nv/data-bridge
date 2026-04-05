import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { inject } from '@angular/core';
import { Observable, tap } from 'rxjs';
import { INTENT } from '../enums';
import { TablesService } from './tables.service';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { MoveResponse, SearchRequest } from './interfaces/search-request';

@Injectable({ providedIn: 'root' })
export class AppService {
  intent = signal(INTENT.NONE);
  http = inject(HttpClient);
  tableService = inject(TablesService);

  snackBarService = inject(MatSnackBar);

  handleAction(action: 'move' | 'search', payload: any): Observable<any> {
    if (action === 'move') {
      return this.tableService.moveData(payload).pipe(
        tap((res) => {
          this.snackBarService.open(`Success! Moved  ${res.moved} rows`);
        }),
      );
    } else {
      return this.tableService.search(payload).pipe(
        tap(() => {
          this.snackBarService.open(`Success!`);
        }),
      );
    }
  }
}
