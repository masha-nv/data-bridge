import { Component, computed, effect, inject, input } from '@angular/core';
import { ENVIRONMENT } from '../../enums';
import { MatTableModule } from '@angular/material/table';
import { CommonModule } from '@angular/common';
import { TablesService } from '../../services/tables.service';
@Component({
  selector: 'app-users',
  templateUrl: './users.component.html',
  styleUrl: './users.component.scss',
  standalone: true,
  imports: [MatTableModule, CommonModule],
})
export class UsersComponent {
  tableService = inject(TablesService);
  env = input.required<ENVIRONMENT>();
  users = computed(() => this.tableService.searchResults());

  displayedColumns = ['user_id', 'user_name'];
}
