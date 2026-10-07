-- Entirely invented household, fixed dates and integer ZAR cents. Never owner exports.
BEGIN;
DELETE FROM builtin_rules;
DELETE FROM categories;
DELETE FROM periods;
UPDATE workspace_branding SET display_name='Demo household';
INSERT INTO accounts(id,name,bank_id,household,balance_cents,balance_date) VALUES
(1,'Everyday account','DEMO-EVERYDAY',1,1842500,'2026-10-07'),
(2,'Rainy day savings','DEMO-SAVINGS',1,4200000,'2026-10-07'),
(3,'Personal spending','DEMO-PRIVATE',0,235000,'2026-10-07');
INSERT INTO grants(user_id,account_id,role) VALUES(1,3,'editor');
INSERT INTO categories(id,name,group_name,kind) VALUES
(1,'Groceries','','expense'),(2,'Transport','','expense'),(3,'Eating out','','expense'),
(4,'Rent','','expense'),(5,'Electricity','','expense'),(6,'Salary','','income');
INSERT INTO periods(id,name,start_date,end_date) VALUES
(1,'September 2026','2026-08-20','2026-09-19'),
(2,'October 2026','2026-09-20','2026-10-19'),
(3,'November 2026','2026-10-20','2026-11-19');
INSERT INTO budget_groups(period_id,spending_group_id)
SELECT p.id,g.id FROM periods p CROSS JOIN spending_groups g WHERE g.name IN ('Day-to-day','Recurring');
INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents)
SELECT p.id,c.id,g.id,CASE c.id WHEN 1 THEN 500000 WHEN 2 THEN 220000 WHEN 3 THEN 150000 WHEN 4 THEN 950000 WHEN 5 THEN 180000 END
FROM periods p CROSS JOIN categories c JOIN spending_groups g ON g.name=CASE WHEN c.id IN (4,5) THEN 'Recurring' ELSE 'Day-to-day' END WHERE c.id<=5;
INSERT INTO targets SELECT period_id,category_id,SUM(amount_cents) FROM group_targets GROUP BY period_id,category_id;
INSERT INTO rules(user_id,account_id,pattern,category_id,spending_group_id,direction)
SELECT 1,1,'Demo Market',1,id,'debit' FROM spending_groups WHERE name='Day-to-day';
-- Replicate the same invented scenario across two periods for trends.
WITH samples(day,amount,description,category,group_name) AS (VALUES
('20',3800000,'Demo Studio salary',6,'Income'),
('21',-950000,'Demo Apartments rent',4,'Recurring'),
('22',-156000,'Demo Electricity prepaid',5,'Recurring'),
('23',-84650,'Demo Market groceries',1,'Day-to-day'),
('25',-62000,'Demo Fuel station',2,'Day-to-day'),
('27',-28500,'Demo Corner cafe',3,'Day-to-day'),
('28',12500,'Demo Market refund',1,'Day-to-day'))
INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,review_state,spending_group_id,period_id)
SELECT 1,substr(p.start_date,1,8)||s.day,s.amount,s.description,substr(p.start_date,1,8)||s.day,s.amount,s.description,'{"synthetic":true,"fixture":"household-v1"}','approved',g.id,p.id
FROM periods p CROSS JOIN samples s JOIN spending_groups g ON g.name=s.group_name WHERE p.id<=2;
INSERT INTO allocations(transaction_id,category_id,amount_cents)
SELECT id,CASE WHEN description LIKE '%salary' THEN 6 WHEN description LIKE '%rent' THEN 4 WHEN description LIKE '%prepaid' THEN 5 WHEN description LIKE '%Fuel%' THEN 2 WHEN description LIKE '%cafe' THEN 3 ELSE 1 END,amount_cents FROM transactions;
INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,review_state,period_id,is_transfer,spending_group_id) VALUES
(100,1,'2026-10-01',-97500,'Demo mixed shopping basket','2026-10-01',-97500,'Demo mixed shopping basket','{"synthetic":true}','approved',2,0,(SELECT id FROM spending_groups WHERE name='Day-to-day')),
(101,1,'2026-10-03',-43200,'Demo purchase needing a category','2026-10-03',-43200,'Demo purchase needing a category','{"synthetic":true}','pending_review',2,0,NULL),
(102,1,'2026-10-04',-250000,'Demo savings transfer','2026-10-04',-250000,'Demo savings transfer','{"synthetic":true}','approved',2,1,(SELECT id FROM spending_groups WHERE name='Transfer')),
(103,2,'2026-10-04',250000,'Demo savings transfer received','2026-10-04',250000,'Demo savings transfer received','{"synthetic":true}','approved',2,1,(SELECT id FROM spending_groups WHERE name='Transfer')),
(104,3,'2026-10-05',-18500,'Demo private lunch','2026-10-05',-18500,'Demo private lunch','{"synthetic":true}','approved',NULL,0,(SELECT id FROM spending_groups WHERE name='Day-to-day'));
INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES
(100,1,-72500),(100,3,-25000),(101,NULL,-43200),(102,NULL,-250000),(103,NULL,250000),(104,3,-18500);
INSERT INTO transfer_links VALUES(102,103);
INSERT INTO transaction_seen(user_id,transaction_id,transaction_version) SELECT 1,id,version FROM transactions WHERE id%2=0 AND review_state='approved';
INSERT INTO audit(user_id,entity,action,details) VALUES(1,'demo','seed','{"fixture":"household-v1","synthetic":true}');
COMMIT;
